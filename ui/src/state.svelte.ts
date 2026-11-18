// One status shared by the card line, the section and the page, read from the
// module and kept for a minute, so twenty printer cards make one request.
import type { Api, Channel, Offer, Status } from './api';

/** How long a status is reused before it is read again. */
const FRESH_MS = 60_000;

export class StatusStore {
  readonly #api: Api;

  constructor(api: Api) {
    this.#api = api;
  }

  status = $state<Status | undefined>(undefined);
  error = $state<string | undefined>(undefined);
  loading = $state(false);

  /** Every model asked for so far; a read always covers all of them. */
  #models: string[] = [];
  #readAt = 0;
  #pending: Promise<void> | undefined;

  /** Reads the status when it is stale or lacks one of these models. */
  ensure(models: readonly string[]): Promise<void> {
    const missing = models.some((m) => m !== '' && !this.#models.includes(m));
    for (const m of models) {
      if (m !== '' && !this.#models.includes(m)) this.#models.push(m);
    }
    if (!missing && Date.now() - this.#readAt < FRESH_MS && this.status) {
      return Promise.resolve();
    }
    return this.reload();
  }

  reload(): Promise<void> {
    if (this.#pending) return this.#pending;
    this.loading = true;
    this.#pending = this.#api
      .status([...this.#models])
      .then((s) => {
        this.status = s;
        this.error = undefined;
        this.#readAt = Date.now();
      })
      .catch((err: unknown) => {
        this.error = err instanceof Error ? err.message : String(err);
      })
      .finally(() => {
        this.loading = false;
        this.#pending = undefined;
      });
    return this.#pending;
  }
}

interface ParsedVersion {
  parts: number[];
}

const STAGES: Record<string, number> = { alpha: 0, a: 0, beta: 1, b: 1, rc: 2 };

/** Reads 6.4.0, 6.5.0-RC1, v8.0.3-beta, 8.0.2+16905. */
export const parseVersion = (
  text: string | undefined,
): ParsedVersion | undefined => {
  if (!text) return undefined;
  const base = text.trim().split('+')[0] ?? '';
  const m = /^v?(\d+)\.(\d+)(?:\.(\d+))?(?:[-_.]?([A-Za-z]+)[-_.]?(\d*))?/.exec(
    base,
  );
  if (!m) return undefined;
  const stageName = (m[4] ?? '').toLowerCase();
  const stage = stageName === '' ? 3 : (STAGES[stageName] ?? 0);
  return {
    parts: [
      Number(m[1]),
      Number(m[2]),
      Number(m[3] ?? 0),
      stage,
      m[5] === undefined || m[5] === '' ? 0 : Number(m[5]),
    ],
  };
};

/** -1, 0 or 1; undefined when either side is not a version. */
export const compareVersions = (
  a: string | undefined,
  b: string | undefined,
) => {
  const pa = parseVersion(a);
  const pb = parseVersion(b);
  if (!pa || !pb) return undefined;
  for (let i = 0; i < pa.parts.length; i++) {
    const x = pa.parts[i] ?? 0;
    const y = pb.parts[i] ?? 0;
    if (x !== y) return x < y ? -1 : 1;
  }
  return 0;
};

export interface Verdict {
  /** The channel in force, and whether it was chosen for this printer. */
  channel: Channel;
  chosen: boolean;
  /** The newest firmware on that channel, if any. */
  offer: NonNullable<Offer> | undefined;
  /** 'update' when the offer is newer than the installed firmware. */
  kind: 'update' | 'current' | 'unknown' | 'unsupported';
}

export const verdictFor = (
  status: Status | undefined,
  deviceId: string,
  model: string | undefined,
  installed: string | undefined,
): Verdict => {
  const chosenChannel = status?.channels[deviceId];
  const channel: Channel = chosenChannel ?? status?.default_channel ?? 'stable';
  const offers = model ? status?.models[model] : undefined;
  const offer =
    (channel === 'beta' ? offers?.beta : offers?.stable) ?? undefined;

  let kind: Verdict['kind'] = 'unknown';
  if (status && offers && !offers.stable && !offers.beta) kind = 'unsupported';
  else if (offer) {
    const cmp = compareVersions(installed, offer.version);
    if (cmp === undefined) kind = 'unknown';
    else kind = cmp < 0 ? 'update' : 'current';
  } else if (status && offers) kind = 'current';

  return { channel, chosen: chosenChannel !== undefined, offer, kind };
};
