// The host this module was handed and the API built on it, held for the
// components to read.
//
// A module-level holder and not Svelte context: the setup function runs before
// any component of this module is mounted, so there is no component for
// `setContext` to hang on. Both are set once, in `index.ts`, before anything
// can be drawn, and let go when the person who signed in leaves.
import type { ModuleHost } from '@prusa/connect-ui/slots';

import { type Api, apiOn } from './api';
import { StatusStore } from './state.svelte';

interface Held {
  host: ModuleHost;
  api: Api;
  status: StatusStore;
}

let held: Held | undefined;

/** Called once by the module's setup function, with the host it was given. */
export const holdHost = (host: ModuleHost): void => {
  if (host.signal.aborted) return;
  const api = apiOn(host);
  const current: Held = { host, api, status: new StatusStore(api) };
  held = current;
  host.signal.addEventListener(
    'abort',
    () => {
      if (held === current) held = undefined;
    },
    { once: true },
  );
};

const holding = (): Held => {
  if (!held) {
    throw new Error('the firmware-updates module drew before its setup ran');
  }

  return held;
};

/** The host a component reads. It throws rather than drawing an empty view. */
export const heldHost = (): ModuleHost => holding().host;

/** This module's routes and the two core calls an update takes. */
export const heldApi = (): Api => holding().api;

/** The status shared by every view of this module. */
export const heldStatus = (): StatusStore => holding().status;
