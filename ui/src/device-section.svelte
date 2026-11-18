<script lang="ts">
  import {
    Badge,
    Button,
    ConfirmDialog,
    ToggleGroup,
    ToggleGroupItem,
  } from '@prusa/connect-ui/primitives';

  import type { Channel } from './api';
  import { heldApi, heldHost, heldStatus } from './host';
  import { verdictFor } from './state.svelte';

  interface Props {
    deviceId: string;
  }

  let { deviceId }: Props = $props();

  const host = heldHost();
  const api = heldApi();
  const statusStore = heldStatus();
  const t = host.t;
  const device = $derived(host.fleet.device(deviceId));

  $effect(() => {
    if (device?.model) void statusStore.ensure([device.model]);
  });

  const status = $derived(statusStore.status);
  const verdict = $derived(
    verdictFor(status, deviceId, device?.model, device?.firmware),
  );
  const channelChoice = $derived(status?.channels[deviceId] ?? 'default');

  let busy = $state<'channel' | 'transfer' | 'flash' | undefined>(undefined);
  let message = $state<string | undefined>(undefined);
  let failure = $state<string | undefined>(undefined);
  let onPrinter = $state<{ path: string; version: string } | undefined>(
    undefined,
  );
  let confirmTransfer = $state(false);
  let confirmFlash = $state(false);
  let showNotes = $state(false);

  const chooseChannel = async (value: string) => {
    if (!value || value === channelChoice) return;
    busy = 'channel';
    failure = undefined;
    try {
      await api.setChannel(
        deviceId,
        value === 'default' ? '' : (value as Channel),
      );
      await statusStore.reload();
    } catch (err) {
      failure = err instanceof Error ? err.message : String(err);
    } finally {
      busy = undefined;
    }
  };

  const doTransfer = async () => {
    const offer = verdict.offer;
    if (!device?.model || !offer) return;
    busy = 'transfer';
    failure = undefined;
    message = t('firmware-updates.section.transferring');
    try {
      const path = await api.transfer(deviceId, device.model, offer);
      onPrinter = { path, version: offer.version };
      message = t('firmware-updates.section.transferred', { path });
    } catch (err) {
      message = undefined;
      failure = err instanceof Error ? err.message : String(err);
    } finally {
      busy = undefined;
    }
  };

  const doFlash = async () => {
    if (!onPrinter) return;
    busy = 'flash';
    failure = undefined;
    try {
      await api.flash(deviceId, onPrinter.path);
      message = t('firmware-updates.section.flashing');
      onPrinter = undefined;
    } catch (err) {
      failure = err instanceof Error ? err.message : String(err);
    } finally {
      busy = undefined;
    }
  };

  const date = (iso: string | undefined) =>
    iso ? new Date(iso).toLocaleDateString() : '';
</script>

{#if device}
  <div class="flex flex-col gap-3 text-sm">
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-muted-foreground"
        >{t('firmware-updates.section.installed')}</span
      >
      <span class="font-medium">{device.firmware || '–'}</span>
      {#if verdict.kind === 'update'}
        <Badge variant="warning">{t('firmware-updates.badge.update')}</Badge>
      {:else if verdict.kind === 'current'}
        <Badge variant="success">{t('firmware-updates.badge.current')}</Badge>
      {:else if verdict.kind === 'unsupported'}
        <Badge variant="outline"
          >{t('firmware-updates.badge.unsupported')}</Badge
        >
      {/if}
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <span class="text-muted-foreground"
        >{t('firmware-updates.section.channel')}</span
      >
      <ToggleGroup
        type="single"
        size="sm"
        aria-label={t('firmware-updates.section.channel')}
        value={channelChoice}
        disabled={busy !== undefined}
        onValueChange={(value: string) => void chooseChannel(value)}
      >
        <ToggleGroupItem value="default"
          >{t('firmware-updates.channel.default', {
            channel: t(
              `firmware-updates.channel.${status?.default_channel ?? 'stable'}`,
            ),
          })}</ToggleGroupItem
        >
        <ToggleGroupItem value="stable"
          >{t('firmware-updates.channel.stable')}</ToggleGroupItem
        >
        <ToggleGroupItem value="beta"
          >{t('firmware-updates.channel.beta')}</ToggleGroupItem
        >
      </ToggleGroup>
    </div>
    {#if verdict.channel === 'beta'}
      <p class="text-xs text-muted-foreground">
        {t('firmware-updates.section.betaHint')}
      </p>
    {/if}

    {#if verdict.offer}
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-muted-foreground"
          >{t('firmware-updates.section.newest')}</span
        >
        <span class="font-medium">{verdict.offer.version}</span>
        {#if verdict.offer.prerelease}
          <Badge variant="info">{t('firmware-updates.badge.prerelease')}</Badge>
        {/if}
        <span class="text-muted-foreground"
          >{date(verdict.offer.published)}</span
        >
        <a
          class="text-link underline-offset-4 hover:underline"
          href={verdict.offer.page}
          target="_blank"
          rel="noreferrer">{t('firmware-updates.section.onGithub')}</a
        >
        <Button variant="link" onclick={() => (showNotes = !showNotes)}
          >{showNotes
            ? t('firmware-updates.section.hideNotes')
            : t('firmware-updates.section.showNotes')}</Button
        >
      </div>
      {#if showNotes}
        <pre
          class="max-h-80 overflow-auto rounded-sm border bg-muted p-3 text-xs whitespace-pre-wrap">{verdict
            .offer.notes}</pre>
      {/if}
    {:else if status && verdict.kind !== 'unsupported'}
      <p class="text-muted-foreground">
        {t('firmware-updates.section.noOffer')}
      </p>
    {/if}

    {#if verdict.offer && (verdict.kind === 'update' || verdict.kind === 'current' || verdict.kind === 'unknown')}
      <div class="flex flex-wrap items-center gap-2">
        <Button
          size="sm"
          variant={verdict.kind === 'update' ? 'default' : 'secondary'}
          disabled={busy !== undefined || !device.connected}
          onclick={() => (confirmTransfer = true)}
          >{busy === 'transfer'
            ? t('firmware-updates.section.transferring')
            : t('firmware-updates.section.transfer', {
                version: verdict.offer.version,
              })}</Button
        >
        {#if onPrinter}
          <Button
            size="sm"
            variant="destructive"
            disabled={busy !== undefined || !device.connected}
            onclick={() => (confirmFlash = true)}
            >{t('firmware-updates.section.flash', {
              version: onPrinter.version,
            })}</Button
          >
        {/if}
      </div>
      {#if !device.connected}
        <p class="text-xs text-muted-foreground">
          {t('firmware-updates.section.offline')}
        </p>
      {/if}
    {/if}

    {#if message}
      <p class="text-muted-foreground">{message}</p>
    {/if}
    {#if failure}
      <p class="text-destructive-text">
        {t('firmware-updates.section.failed', { reason: failure })}
      </p>
    {/if}
  </div>

  <ConfirmDialog
    bind:open={confirmTransfer}
    variant="confirm"
    title={t('firmware-updates.confirmTransfer.title')}
    description={t('firmware-updates.confirmTransfer.body', {
      version: verdict.offer?.version ?? '',
      name: device.name !== '' ? device.name : deviceId,
    })}
    confirmLabel={t('firmware-updates.confirmTransfer.confirm')}
    onConfirm={() => {
      confirmTransfer = false;
      void doTransfer();
    }}
  />

  <ConfirmDialog
    bind:open={confirmFlash}
    title={t('firmware-updates.confirmFlash.title')}
    description={t('firmware-updates.confirmFlash.body', {
      version: onPrinter?.version ?? '',
      name: device.name !== '' ? device.name : deviceId,
    })}
    confirmLabel={t('firmware-updates.confirmFlash.confirm')}
    onConfirm={() => {
      confirmFlash = false;
      void doFlash();
    }}
  />
{/if}
