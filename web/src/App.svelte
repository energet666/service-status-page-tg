<script>
  import {
    AlertCircle,
    ChevronDown,
    CheckCircle2,
    Globe2,
    LoaderCircle,
    Megaphone,
    MessageSquareText,
    RefreshCw,
    Send,
    ShieldAlert,
    Wrench
  } from 'lucide-svelte';

  const savedNameKey = 'service-status-page.reportName';
  const checksOpenKey = 'service-status-page.checksOpen';

  let status = $state(null);
  let announcements = $state([]);
  let loading = $state(true);
  let loadError = $state('');
  let checkTargets = $state([]);
  let checksLoading = $state(true);
  let checksError = $state('');
  let checksGeneratedAt = $state('');
  let checksOpen = $state(true);
  let live = $state(false);
  let realtimeError = $state('');
  let reportFormOpen = $state(false);
  let hideUserMessages = $state(false);
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
  let visibleAnnouncements = $derived(announcements.filter((announcement) => !hideUserMessages || announcement.kind !== 'user'));

  $effect(() => {
    loadSavedName();
    loadChecksOpen();
    loadStatus();
    loadChecks();
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

  async function loadChecks() {
    checksLoading = true;
    checksError = '';
    try {
      const response = await fetch('/api/checks');
      const data = await readJSON(response);
      checkTargets = data.targets ?? [];
      checksGeneratedAt = data.meta?.generatedAt ?? '';
    } catch (error) {
      checksError = error.message || 'Не удалось проверить адреса';
    } finally {
      checksLoading = false;
    }
  }

  async function submitReport() {
    submitError = '';
    if (!form.message.trim()) {
      submitError = 'Опишите проблему';
      return;
    }

    const submittedName = form.name.trim();
    submitting = true;
    try {
      await readJSON(
        await fetch('/api/reports', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(form)
        })
      );
      saveName(submittedName);
      form = { message: '', name: submittedName, contact: '' };
      reportFormOpen = false;
      showToast('Спасибо. Сообщение отправлено администратору.');
    } catch (error) {
      submitError = error.message || 'Не удалось отправить сообщение';
    } finally {
      submitting = false;
    }
  }

  function loadSavedName() {
    try {
      const savedName = localStorage.getItem(savedNameKey);
      if (savedName && !form.name) {
        form.name = savedName;
      }
    } catch {
      // Browser storage can be unavailable in private or restricted modes.
    }
  }

  function loadChecksOpen() {
    try {
      const savedValue = localStorage.getItem(checksOpenKey);
      if (savedValue !== null) {
        checksOpen = savedValue === 'true';
      }
    } catch {
      // Browser storage can be unavailable in private or restricted modes.
    }
  }

  function toggleChecksOpen() {
    checksOpen = !checksOpen;
    try {
      localStorage.setItem(checksOpenKey, String(checksOpen));
    } catch {
      // The UI state can still change even if it cannot be persisted.
    }
  }

  function saveName(name) {
    if (!name) return;
    try {
      localStorage.setItem(savedNameKey, name);
    } catch {
      // Sending the report is more important than persisting the form default.
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

  function getCheckView(state) {
    if (state === 'up') {
      return {
        title: 'Доступен',
        badge: 'badge-success',
        border: 'border-success/35',
        tone: 'text-success'
      };
    }
    if (state === 'http_error') {
      return {
        title: 'HTTP ошибка',
        badge: 'badge-warning',
        border: 'border-warning/35',
        tone: 'text-warning'
      };
    }
    return {
      title: 'Недоступен',
      badge: 'badge-error',
      border: 'border-error/35',
      tone: 'text-error'
    };
  }

  function formatLatency(value) {
    if (typeof value !== 'number' || value < 0) return '';
    return `${value} мс`;
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
    if (kind === 'user') return 'border-info/45 border-l-info bg-info/5';
    if (kind === 'maintenance') return 'border-warning/45 border-l-warning bg-warning/5';
    if (kind === 'incident') return 'border-error/45 border-l-error bg-error/5';
    if (kind === 'resolved') return 'border-success/45 border-l-success bg-success/5';
    return 'border-base-300/70 border-l-base-content/25';
  }

  function announcementLabel(kind) {
    if (kind === 'user') return 'Сообщение пользователя';
    if (kind === 'maintenance') return 'Обслуживание';
    if (kind === 'incident') return 'Инцидент';
    if (kind === 'resolved') return 'Решено';
    return 'Объявление';
  }

  function userDisplayName(announcement) {
    if (announcement.kind !== 'user') return '';
    return announcement.createdBy && announcement.createdBy !== 'user' ? announcement.createdBy : 'Анонимно';
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

    <section class="surface-panel rounded-lg border border-base-content/10 p-5 sm:p-6">
      <div class={checksOpen ? 'mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between' : 'flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'}>
        <button
          class="flex min-w-0 items-center gap-2 rounded-lg text-left text-base-content hover:text-secondary"
          type="button"
          aria-controls="availability-checks"
          aria-expanded={checksOpen}
          onclick={toggleChecksOpen}
        >
          <Globe2 class="size-5 shrink-0 text-secondary" />
          <span class="text-2xl font-semibold">Проверка доступности</span>
          <ChevronDown class={`size-5 shrink-0 transition-transform ${checksOpen ? 'rotate-180' : ''}`} />
        </button>

        <div class="flex flex-wrap items-center gap-2">
          {#if checksGeneratedAt}
            <span class="text-sm text-base-content/55">Обновлено {formatDate(checksGeneratedAt)}</span>
          {/if}
          {#if checksOpen}
            <button
              class="btn btn-sm rounded-lg border-base-content/15 bg-base-200 text-base-content hover:border-base-content/25 hover:bg-base-300"
              type="button"
              onclick={loadChecks}
              disabled={checksLoading}
            >
              {#if checksLoading}
                <LoaderCircle class="size-4 animate-spin" />
                Проверка
              {:else}
                <RefreshCw class="size-4" />
                Проверить
              {/if}
            </button>
          {/if}
        </div>
      </div>

      {#if checksOpen}
        <div id="availability-checks">
          {#if checksError}
            <div class="alert alert-error rounded-lg">
              <AlertCircle class="size-5" />
              <span>{checksError}</span>
            </div>
          {:else if checksLoading && checkTargets.length === 0}
            <div class="surface-muted flex min-h-28 items-center justify-center rounded-lg border border-base-content/10">
              <LoaderCircle class="size-6 animate-spin" />
            </div>
          {:else if checkTargets.length === 0}
            <div class="surface-muted rounded-lg border border-base-content/10 p-4 text-base-content/70">
              Адреса для проверки не настроены.
            </div>
          {:else}
            <div class="grid gap-3 md:grid-cols-2">
              {#each checkTargets as target}
                {@const checkView = getCheckView(target.state)}
                <article class={`surface-muted rounded-lg border p-4 ${checkView.border}`}>
                  <div class="flex flex-col gap-3">
                    <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                      <div class="min-w-0">
                        <h3 class="font-semibold leading-snug">{target.name}</h3>
                        <a class="break-all text-sm text-base-content/65 hover:text-secondary" href={target.url} target="_blank" rel="noreferrer">
                          {target.url}
                        </a>
                      </div>
                      <span class={`badge shrink-0 rounded-lg ${checkView.badge}`}>{checkView.title}</span>
                    </div>

                    <div class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-base-content/70">
                      {#if formatLatency(target.latencyMs)}
                        <span>Задержка {formatLatency(target.latencyMs)}</span>
                      {/if}
                      {#if target.statusCode}
                        <span>HTTP {target.statusCode}</span>
                      {/if}
                      {#if target.checkedAt}
                        <span>Проверено {formatDate(target.checkedAt)}</span>
                      {/if}
                    </div>

                    {#if target.error}
                      <p class={`text-sm leading-relaxed ${checkView.tone}`}>{target.error}</p>
                    {/if}
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
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

    <div class={reportFormOpen ? 'grid gap-8 lg:grid-cols-[1.2fr_0.8fr] lg:items-start' : 'grid gap-8'}>
      {#if reportFormOpen}
        <section id="report-form" class="surface-panel rounded-lg border border-base-content/10 p-5 lg:order-2">
          <div class="mb-4 flex items-center gap-2">
            <Send class="size-5 text-primary" />
            <h2 class="text-2xl font-semibold">Сообщить о баге</h2>
          </div>
          <p class="mb-4 text-sm leading-relaxed text-base-content/65">
            Текст сообщения появится в чате статуса. Имя и контакт видны только администратору.
          </p>

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
        <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex items-center gap-2">
            <Megaphone class="size-5 text-accent" />
            <h2 class="text-2xl font-semibold">Чат статуса</h2>
          </div>

          <label class="flex items-center gap-2 text-sm text-base-content/75">
            <input class="checkbox checkbox-sm rounded border-base-content/25" type="checkbox" bind:checked={hideUserMessages} />
            <span>Скрыть сообщения пользователей</span>
          </label>
        </div>

        {#if loading && announcements.length === 0}
          <div class="surface-muted flex min-h-40 items-center justify-center rounded-lg border border-base-content/10">
            <LoaderCircle class="size-6 animate-spin" />
          </div>
        {:else if announcements.length === 0}
          <div class="surface-muted rounded-lg border border-base-content/10 p-5 text-base-content/70">
            Сообщений пока нет.
          </div>
        {:else if visibleAnnouncements.length === 0}
          <div class="surface-muted rounded-lg border border-base-content/10 p-5 text-base-content/70">
            Пользовательские сообщения скрыты.
          </div>
        {:else}
          <div class="space-y-3">
            {#each visibleAnnouncements as announcement}
              <article class={`surface-muted rounded-lg border border-l-4 p-4 ${announcementClass(announcement.kind)}`}>
                <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <div class="min-w-0">
                    <div class="mb-2 flex flex-wrap items-center gap-2">
                      {#if announcement.kind === 'user'}
                        <MessageSquareText class="size-4 text-info" />
                      {/if}
                      <span class={`badge badge-sm rounded-lg ${announcement.kind === 'user' ? 'badge-info' : 'badge-ghost'}`}>
                        {announcementLabel(announcement.kind)}
                      </span>
                      {#if announcement.kind === 'user'}
                        <span class="text-sm font-medium text-base-content/65">{userDisplayName(announcement)}</span>
                      {/if}
                    </div>
                    <p class="leading-relaxed">{announcement.message}</p>
                  </div>
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
