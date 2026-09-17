// 1 when running inside an app bundle (required by UNUserNotificationCenter).
int  mmNotifyAvailable(void);
// Installs the delegate and the "Don't alert for this app" action, and asks
// for permission up front when requestAuth is set.
void mmNotifyInit(int requestAuth);
// Requests authorization if needed and posts a notification. The outcome is
// reported asynchronously via mmNotifyDone(reqID, error-or-NULL).
void mmNotifySend(long long reqID, const char* ident, const char* title, const char* body,
                  const char* category, const char* subject);
