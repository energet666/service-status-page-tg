<script>
  import {
    AlertCircle,
    CheckCircle2,
    LoaderCircle,
    Megaphone,
    RefreshCw,
    Send,
    ShieldAlert,
    Wrench
  } from 'lucide-svelte';

  let status = $state(null);
  let announcements = $state([]);
  let loading = $state(true);
  let loadError = $state('');
  let live = $state(false);
  let realtimeError = $state('');
  let reportFormOpen = $state(false);
  let submitting = $state(false);
  let submitError = $state('');
  let toastMessage = $state('');
  let toastTimer;
  let form = $state({
    message: '',
    name: '',
    contact: ''
  });

  let statusView = $derived(getStatusView(status?.state));
  let StatusIcon = $derived(statusView.icon);

  $effect(() => {
    loadStatus();
    const events = new EventSource('/api/status/events');

    events.addEventListener('status', (event) => {
      try {
        applyStatusData(JSON.parse(event.data));
        live = true;
        realtimeError = '';
      } catch {
        realtimeError = 'Не удалось прочитать обновление статуса';
      }
    });

    events.onerror = () => {
      live = false;
      if (!status) {
        realtimeError = 'Автообновление временно недоступно';
      }
    };

    return () => {
      events.close();
    };
  });

  async function loadStatus() {
    loading = true;
    loadError = '';
    try {
      const response = await fetch('/api/status');
      const data = await readJSON(response);
      applyStatusData(data);
    } catch (error) {
      loadError = error.message || 'Не удалось загрузить статус';
    } finally {
      loading = false;
    }
  }

  function applyStatusData(data) {
    status = data.status;
    announcements = data.announcements ?? [];
    loading = false;
    loadError = '';
  }

  async function submitReport() {
    submitError = '';
    if (!form.message.trim()) {
      submitError = 'Опишите проблему';
      return;
    }

    submitting = true;
    try {
      await readJSON(
        await fetch('/api/reports', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(form)
        })
      );
      form = { message: '', name: '', contact: '' };
      reportFormOpen = false;
      showToast('Спасибо. Сообщение отправлено администратору.');
    } catch (error) {
      submitError = error.message || 'Не удалось отправить сообщение';
    } finally {
      submitting = false;
    }
  }

  async function readJSON(response) {
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(data.error || 'Ошибка запроса');
    }
    return data;
  }

  function showToast(message) {
    toastMessage = message;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toastMessage = '';
    }, 3500);
  }

  function getStatusView(state) {
    if (state === 'maintenance') {
      return {
        title: 'Идет обслуживание',
        badge: 'badge-warning',
        panel: 'border-warning/35 bg-warning/10',
        tone: 'text-warning',
        icon: Wrench
      };
    }
    if (state === 'incident') {
      return {
        title: 'Есть инцидент',
        badge: 'badge-error',
        panel: 'border-error/35 bg-error/10',
        tone: 'text-error',
        icon: ShieldAlert
      };
    }
    return {
      title: 'Работает штатно',
      badge: 'badge-success',
      panel: 'border-success/35 bg-success/10',
      tone: 'text-success',
      icon: CheckCircle2
    };
  }

  function formatDate(value) {
    if (!value) return '';
    return new Intl.DateTimeFormat('ru-RU', {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit'
    }).format(new Date(value));
  }

  function announcementClass(kind) {
    if (kind === 'maintenance') return 'border-warning/45 border-l-warning bg-warning/5';
    if (kind === 'incident') return 'border-error/45 border-l-error bg-error/5';
    if (kind === 'resolved') return 'border-success/45 border-l-success bg-success/5';
    return 'border-base-300/70 border-l-base-content/25';
  }
</script>

<svelte:head>
  <title>Статус сервиса</title>
</svelte:head>

<main class="status-shell min-h-screen bg-base-100 text-base-content">
  {#if toastMessage}
    <div class="toast toast-top toast-center z-50 px-4 pt-4 sm:toast-end">
      <div class="toast-card alert alert-success rounded-lg shadow-lg" role="status" aria-live="polite">
        <CheckCircle2 class="size-5" />
        <span>{toastMessage}</span>
      </div>
    </div>
  {/if}

  <section class="mx-auto flex w-full max-w-5xl flex-col gap-8 px-4 py-8 sm:px-6 lg:px-8 lg:py-12">
    <h1 class="max-w-3xl text-3xl font-semibold leading-tight text-base-content sm:text-4xl">Состояние сервиса</h1>

    <div class="flex flex-wrap items-center gap-2 text-sm text-base-content/65">
      <span class={`badge badge-sm border-base-content/10 rounded-lg ${live ? 'badge-success' : 'badge-ghost bg-base-200/70'}`}>
        {live ? 'Автообновление включено' : 'Подключение к автообновлению'}
      </span>
      <button
        class="btn btn-ghost btn-square btn-sm rounded-lg border-0 bg-transparent text-base-content/70 shadow-none hover:bg-base-content/10"
        type="button"
        aria-label="Обновить статус"
        title="Обновить статус"
        onclick={loadStatus}
        disabled={loading}
      >
        {#if loading}
          <LoaderCircle class="size-4 animate-spin" />
        {:else}
          <RefreshCw class="size-4" />
        {/if}
      </button>
      {#if realtimeError}
        <span>{realtimeError}</span>
      {/if}
    </div>

    {#if loadError}
      <div class="alert alert-error rounded-lg">
        <AlertCircle class="size-5" />
        <span>{loadError}</span>
      </div>
    {/if}

    <section class={`surface-panel rounded-lg border p-5 sm:p-6 ${statusView.panel}`}>
      <div class="flex items-start gap-4">
        <div class={`shrink-0 rounded-lg border border-current/20 bg-base-100/50 p-2 ${statusView.tone}`}>
          <StatusIcon class="size-7" />
        </div>
        <div class="min-w-0 text-base-content">
          <div class={`badge ${statusView.badge} rounded-lg`}>{statusView.title}</div>
          <p class="mt-3 text-xl font-medium leading-snug">{status?.message ?? 'Загрузка статуса...'}</p>
          {#if status?.updatedAt}
            <p class="mt-2 text-sm text-base-content/65">Обновлено {formatDate(status.updatedAt)}</p>
          {/if}
        </div>
      </div>
    </section>

    <div>
      <button
        class="btn w-full rounded-lg border-base-content/15 bg-base-200 text-base-content hover:border-base-content/25 hover:bg-base-300 sm:w-auto"
        type="button"
        aria-controls="report-form"
        aria-expanded={reportFormOpen}
        onclick={() => {
          reportFormOpen = !reportFormOpen;
        }}
      >
        <Send class="size-4" />
        Сообщить о проблеме
      </button>
    </div>

    <div class={reportFormOpen ? 'grid gap-8 lg:grid-cols-[1.2fr_0.8fr]' : 'grid gap-8'}>
      {#if reportFormOpen}
        <section id="report-form" class="surface-panel rounded-lg border border-base-content/10 p-5 lg:order-2">
          <div class="mb-4 flex items-center gap-2">
            <Send class="size-5 text-primary" />
            <h2 class="text-2xl font-semibold">Сообщить о баге</h2>
          </div>

          <form class="flex flex-col gap-4" onsubmit={(event) => { event.preventDefault(); submitReport(); }}>
            <label class="flex flex-col gap-2">
              <span class="text-sm font-medium text-base-content/75">Что случилось</span>
              <textarea
                class="textarea textarea-bordered min-h-32 w-full rounded-lg bg-base-100/70"
                bind:value={form.message}
                placeholder="Опишите проблему"
                maxlength="4000"
              ></textarea>
            </label>

            <label class="flex flex-col gap-2">
              <span class="text-sm font-medium text-base-content/75">Имя</span>
              <input class="input input-bordered w-full rounded-lg bg-base-100/70" bind:value={form.name} placeholder="Необязательно" maxlength="120" />
            </label>

            <label class="flex flex-col gap-2">
              <span class="text-sm font-medium text-base-content/75">Контакт</span>
              <input class="input input-bordered w-full rounded-lg bg-base-100/70" bind:value={form.contact} placeholder="Email или Telegram, необязательно" maxlength="200" />
            </label>

            {#if submitError}
              <div class="alert alert-error rounded-lg py-3">
                <AlertCircle class="size-5" />
                <span>{submitError}</span>
              </div>
            {/if}

            <button class="btn btn-primary w-full rounded-lg" type="submit" disabled={submitting}>
              {#if submitting}
                <LoaderCircle class="size-4 animate-spin" />
                Отправка
              {:else}
                <Send class="size-4" />
                Отправить
              {/if}
            </button>
          </form>
        </section>
      {/if}

      <section class="lg:order-1">
        <div class="mb-4 flex items-center gap-2">
          <Megaphone class="size-5 text-accent" />
          <h2 class="text-2xl font-semibold">Объявления</h2>
        </div>

        {#if loading && announcements.length === 0}
          <div class="surface-muted flex min-h-40 items-center justify-center rounded-lg border border-base-content/10">
            <LoaderCircle class="size-6 animate-spin" />
          </div>
        {:else if announcements.length === 0}
          <div class="surface-muted rounded-lg border border-base-content/10 p-5 text-base-content/70">
            Объявлений пока нет.
          </div>
        {:else}
          <div class="space-y-3">
            {#each announcements as announcement}
              <article class={`surface-muted rounded-lg border border-l-4 p-4 ${announcementClass(announcement.kind)}`}>
                <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <p class="leading-relaxed">{announcement.message}</p>
                  <time class="shrink-0 text-sm text-base-content/60">{formatDate(announcement.createdAt)}</time>
                </div>
              </article>
            {/each}
          </div>
        {/if}
      </section>
    </div>
  </section>
</main>
