// service worker — showNotification for app_hub pushes
self.registration.showNotification(title, {
  body,
  icon: '/icons/icon-192.png',   // full-colour app icon
  badge: '/icons/badge-96.png',   // monochrome, alpha only (Android status bar)
  data,
});
