<script lang="ts">
  import { heldHost, heldStatus } from './host';
  import { verdictFor } from './state.svelte';

  interface Props {
    deviceId: string;
  }

  let { deviceId }: Props = $props();

  const host = heldHost();
  const statusStore = heldStatus();
  const t = host.t;
  const device = $derived(host.fleet.device(deviceId));

  $effect(() => {
    if (device?.model) void statusStore.ensure([device.model]);
  });

  const verdict = $derived(
    verdictFor(statusStore.status, deviceId, device?.model, device?.firmware),
  );
</script>

<!-- One line on the printer's card: only when there is something to say. -->
{#if device && verdict.kind === 'update' && verdict.offer}
  <span class="text-sm font-medium text-warning-foreground"
    >{t('firmware-updates.card.update', {
      version: verdict.offer.version,
    })}</span
  >
{:else if device && verdict.kind === 'current'}
  <span class="text-sm text-muted-foreground"
    >{t('firmware-updates.card.current', {
      version: device.firmware,
    })}</span
  >
{/if}
