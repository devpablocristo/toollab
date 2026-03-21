import { createBrowserStorageNamespace } from '@devpablocristo/core-browser/storage'

export const appStorage = createBrowserStorageNamespace({
  namespace: 'toollab',
  hostAware: false,
})
