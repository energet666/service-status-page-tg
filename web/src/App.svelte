<script>
  import {
    AlertCircle,
    CheckCircle2,
    Clock,
    LoaderCircle,
    Megaphone,
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
  let submitting = $state(false);
  let submitError = $state('');
  let submitSuccess = $state('');
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
    submitSuccess = '';
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
      submitSuccess = 'Спасибо. Сообщение отправлено администратору.';
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

  function getStatusView(state) {
    if (state === 'maintenance') {
      return {
        title: 'Идет обслуживание',
        badge: 'badge-warning',
        panel: 'border-warning/30 bg-warning/10',
        icon: Wrench
      };
    }
    if (state === 'incident') {
      return {
        title: 'Есть инцидент',
        badge: 'badge-error',
        panel: 'border-error/30 bg-error/10',
        icon: ShieldAlert
      };
    }
    return {
      title: 'Работает штатно',
      badge: 'badge-success',
      panel: 'border-success/30 bg-success/10',
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
    if (kind === 'maintenance') return 'border-warning/30';
    if (kind === 'incident') return 'border-error/30';
    if (kind === 'resolved') return 'border-success/30';
    return 'border-base-300';
  }
</script>

<svelte:head>
  <title>Статус сервиса</title>
</svelte:head>

<main class="min-h-screen bg-base-100 text-base-content">
  <section class="mx-auto flex w-full max-w-5xl flex-col gap-8 px-4 py-8 sm:px-6 lg:px-8">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <p class="text-sm font-medium text-base-content/60">Статус сервиса</p>
        <h1 class="mt-1 text-3xl font-semibold leading-tight sm:text-4xl">Проверка доступности и объявления</h1>
      </div>
      <button class="btn btn-outline btn-sm rounded-lg" onclick={loadStatus} disabled={loading}>
        {#if loading}
          <LoaderCircle class="size-4 animate-spin" />
        {:else}
          <Clock class="size-4" />
        {/if}
        Обновить
      </button>
    </div>

    <div class="flex flex-wrap items-center gap-2 text-sm text-base-content/60">
      <span class={`badge badge-sm rounded-lg ${live ? 'badge-success' : 'badge-ghost'}`}>
        {live ? 'Автообновление включено' : 'Подключение к автообновлению'}
      </span>
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

    <section class={`rounded-lg border p-5 ${statusView.panel}`}>
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div class="flex gap-3">
          <div class="mt-1">
            <StatusIcon class="size-7" />
          </div>
          <div>
            <div class={`badge ${statusView.badge} rounded-lg`}>{statusView.title}</div>
            <p class="mt-3 text-xl font-medium leading-snug">{status?.message ?? 'Загрузка статуса...'}</p>
            {#if status?.updatedAt}
              <p class="mt-2 text-sm text-base-content/65">Обновлено {formatDate(status.updatedAt)}</p>
            {/if}
          </div>
        </div>
      </div>
    </section>

    <div class="grid gap-8 lg:grid-cols-[1.2fr_0.8fr]">
      <section>
        <div class="mb-4 flex items-center gap-2">
          <Megaphone class="size-5" />
          <h2 class="text-2xl font-semibold">Объявления</h2>
        </div>

        {#if loading && announcements.length === 0}
          <div class="flex min-h-40 items-center justify-center rounded-lg border border-base-300">
            <LoaderCircle class="size-6 animate-spin" />
          </div>
        {:else if announcements.length === 0}
          <div class="rounded-lg border border-base-300 p-5 text-base-content/70">
            Объявлений пока нет.
          </div>
        {:else}
          <div class="space-y-3">
            {#each announcements as announcement}
              <article class={`rounded-lg border bg-base-100 p-4 ${announcementClass(announcement.kind)}`}>
                <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <p class="leading-relaxed">{announcement.message}</p>
                  <time class="shrink-0 text-sm text-base-content/60">{formatDate(announcement.createdAt)}</time>
                </div>
              </article>
            {/each}
          </div>
        {/if}
      </section>

      <section class="rounded-lg border border-base-300 p-5">
        <div class="mb-4 flex items-center gap-2">
          <Send class="size-5" />
          <h2 class="text-2xl font-semibold">Сообщить о баге</h2>
        </div>

        <form class="flex flex-col gap-4" onsubmit={(event) => { event.preventDefault(); submitReport(); }}>
          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-base-content/75">Что случилось</span>
            <textarea
              class="textarea textarea-bordered min-h-32 w-full rounded-lg"
              bind:value={form.message}
              placeholder="Опишите проблему"
              maxlength="4000"
            ></textarea>
          </label>

          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-base-content/75">Имя</span>
            <input class="input input-bordered w-full rounded-lg" bind:value={form.name} placeholder="Необязательно" maxlength="120" />
          </label>

          <label class="flex flex-col gap-2">
            <span class="text-sm font-medium text-base-content/75">Контакт</span>
            <input class="input input-bordered w-full rounded-lg" bind:value={form.contact} placeholder="Email или Telegram, необязательно" maxlength="200" />
          </label>

          {#if submitError}
            <div class="alert alert-error rounded-lg py-3">
              <AlertCircle class="size-5" />
              <span>{submitError}</span>
            </div>
          {/if}
          {#if submitSuccess}
            <div class="alert alert-success rounded-lg py-3">
              <CheckCircle2 class="size-5" />
              <span>{submitSuccess}</span>
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
    </div>
  </section>
</main>
