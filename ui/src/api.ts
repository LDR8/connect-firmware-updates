// The service layer. This module's own routes are reached through the typed
// client `make client` generates from openapi.json, bound onto the client the
// host handed over, so every call carries the person's session. Sending the
// file to the printer and the FLASH verb are the core's own operations on the
// same client: the core checks the person's permissions for both.
import type { ModuleHost } from '@prusa/connect-ui/slots';
import { bind, type components } from 'firmware-updates-client';

export type Offer = components['schemas']['Offer'];
export type Status = components['schemas']['Status'];
export type Staged = components['schemas']['Staged'];
export type Channel = 'stable' | 'beta';

export interface Api {
  status(models: readonly string[], signal?: AbortSignal): Promise<Status>;
  setChannel(device: string, channel: Channel | ''): Promise<void>;
  refresh(): Promise<void>;
  /** Downloads into the server's storage and copies to the printer's USB drive. Answers the path on the printer. */
  transfer(
    deviceId: string,
    model: string,
    offer: NonNullable<Offer>,
  ): Promise<string>;
  /** Sends FLASH for a file on the printer's USB drive. The printer restarts. */
  flash(deviceId: string, path: string): Promise<void>;
}

export const apiOn = (host: ModuleHost): Api => {
  const routes = bind(host.client);
  const until = (signal?: AbortSignal): AbortSignal =>
    signal === undefined ? host.signal : AbortSignal.any([signal, host.signal]);

  return {
    status: (models, signal) =>
      routes.readFirmwareStatus(
        { query: { models: models.join(',') } },
        until(signal),
      ),
    setChannel: async (device, channel) => {
      await routes.setFirmwareChannel(
        { path: { id: device }, body: { channel } },
        until(),
      );
    },
    refresh: async () => {
      await routes.refreshFirmwareReleases({}, until());
    },
    transfer: async (deviceId, model, offer) => {
      const staged = await routes.stageFirmware(
        { body: { model, tag: offer.tag } },
        until(),
      );
      const sent = await host.client.sendDeviceFile(
        {
          path: { device_id: deviceId },
          body: {
            storage: staged.storage,
            source: staged.path,
            path: `usb/${staged.printer_file}`,
            replace: true,
          },
        },
        until(),
      );
      const landed = (sent as { path?: string }).path;

      return landed !== undefined && landed !== ''
        ? landed
        : `/usb/${staged.printer_file}`;
    },
    flash: async (deviceId, path) => {
      await host.client.sendDeviceCommand(
        {
          path: { device_id: deviceId },
          body: {
            verb: 'FLASH',
            args: { path: path.startsWith('/') ? path : `/${path}` },
          },
        },
        until(),
      );
    },
  };
};
