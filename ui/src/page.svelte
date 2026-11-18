<script lang="ts">
  import {
    Badge,
    Button,
    Card,
    CardContent,
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
    PageHeader,
  } from '@prusa/connect-ui/primitives';

  import { heldApi, heldHost, heldStatus } from './host';
  import { verdictFor } from './state.svelte';

  const host = heldHost();
  const api = heldApi();
  const statusStore = heldStatus();
  const t = host.t;
  const devices = $derived(host.fleet.devices);

  $effect(() => {
    const models = devices.map((d) => d.model).filter((m) => m !== '');
    void statusStore.ensure(models);
  });

  const status = $derived(statusStore.status);
  let checking = $state(false);
  let failure = $state<string | undefined>(undefined);
  const problem = $derived(failure ?? status?.error ?? statusStore.error);

  const checkNow = async () => {
    checking = true;
    failure = undefined;
    try {
      await api.refresh();
      await statusStore.reload();
    } catch (err) {
      failure = err instanceof Error ? err.message : String(err);
    } finally {
      checking = false;
    }
  };

  const when = (iso: string | undefined) =>
    iso ? new Date(iso).toLocaleString() : t('firmware-updates.page.never');
</script>

<div class="flex flex-col gap-4">
  <PageHeader title={t('firmware-updates.page.title')}>
    {#snippet actions()}
      <Button
        size="sm"
        variant="secondary"
        disabled={checking}
        onclick={() => void checkNow()}
        >{checking
          ? t('firmware-updates.page.checking')
          : t('firmware-updates.page.check')}</Button
      >
    {/snippet}
  </PageHeader>

  <p class="text-sm text-muted-foreground">
    {t('firmware-updates.page.intro', {
      channel: t(
        `firmware-updates.channel.${status?.default_channel ?? 'stable'}`,
      ),
    })}
  </p>
  <p class="text-sm text-muted-foreground">
    {t('firmware-updates.page.checked', { when: when(status?.checked) })}
  </p>
  {#if problem !== undefined}
    <p class="text-sm text-destructive-text">
      {t('firmware-updates.section.failed', {
        reason: problem,
      })}
    </p>
  {/if}

  <Card>
    <CardContent>
      {#if devices.length === 0}
        <p class="text-sm text-muted-foreground">
          {t('firmware-updates.page.empty')}
        </p>
      {:else}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('firmware-updates.page.printer')}</TableHead>
              <TableHead>{t('firmware-updates.page.model')}</TableHead>
              <TableHead>{t('firmware-updates.section.installed')}</TableHead>
              <TableHead>{t('firmware-updates.section.channel')}</TableHead>
              <TableHead>{t('firmware-updates.section.newest')}</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {#each devices as device (device.id)}
              {@const v = verdictFor(
                status,
                device.id,
                device.model,
                device.firmware,
              )}
              <TableRow>
                <TableCell>
                  <a
                    class="text-link underline-offset-4 hover:underline"
                    href={`/printers/${encodeURIComponent(device.id)}`}
                    >{device.name !== '' ? device.name : device.id}</a
                  >
                </TableCell>
                <TableCell>{device.model}</TableCell>
                <TableCell
                  >{device.firmware !== '' ? device.firmware : '–'}</TableCell
                >
                <TableCell>
                  {t(`firmware-updates.channel.${v.channel}`)}{v.chosen
                    ? ''
                    : ` (${t('firmware-updates.page.byDefault')})`}
                </TableCell>
                <TableCell>{v.offer?.version ?? '–'}</TableCell>
                <TableCell>
                  {#if v.kind === 'update'}
                    <Badge variant="warning"
                      >{t('firmware-updates.badge.update')}</Badge
                    >
                  {:else if v.kind === 'current'}
                    <Badge variant="success"
                      >{t('firmware-updates.badge.current')}</Badge
                    >
                  {:else if v.kind === 'unsupported'}
                    <Badge variant="outline"
                      >{t('firmware-updates.badge.unsupported')}</Badge
                    >
                  {/if}
                </TableCell>
              </TableRow>
            {/each}
          </TableBody>
        </Table>
      {/if}
    </CardContent>
  </Card>
  <p class="text-xs text-muted-foreground">{t('firmware-updates.page.how')}</p>
</div>
