/* ==========================================================================
   CHORECRAFT SERVICE WORKER — Web Push Notifications
   ========================================================================== */

// Listen for push events from the Push API
self.addEventListener("push", (event) => {
  let data = { title: "ChoreCraft", body: "You have new tasks!" };

  if (event.data) {
    try {
      data = event.data.json();
    } catch (e) {
      data.body = event.data.text();
    }
  }

  const options = {
    body: data.body || "You have new tasks!",
    icon: "icon.svg",
    badge: "icon.svg",
    vibrate: [200, 100, 200],
    tag: "chorecraft-notification",
    renotify: true,
    data: {
      url: self.location.origin
    }
  };

  event.waitUntil(
    self.registration.showNotification(data.title || "ChoreCraft", options)
  );
});

// Handle notification click — focus or open the app
self.addEventListener("notificationclick", (event) => {
  event.notification.close();

  const urlToOpen = event.notification.data?.url || self.location.origin;

  event.waitUntil(
    clients.matchAll({ type: "window", includeUncontrolled: true }).then((clientList) => {
      // Focus an existing tab if available
      for (const client of clientList) {
        if (client.url.startsWith(urlToOpen) && "focus" in client) {
          return client.focus();
        }
      }
      // Otherwise open a new tab
      return clients.openWindow(urlToOpen);
    })
  );
});

// Activate immediately
self.addEventListener("activate", (event) => {
  event.waitUntil(clients.claim());
});
