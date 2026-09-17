#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#include "notify.h"

extern void mmNotifyDone(long long reqID, char* err);
extern void mmNotifyAction(char* action, char* subject);

@interface MMNotifyDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation MMNotifyDelegate
// Show banners even though the app is technically "active" (menu bar app).
- (void)userNotificationCenter:(UNUserNotificationCenter*)center
       willPresentNotification:(UNNotification*)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
                      UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter*)center
    didReceiveNotificationResponse:(UNNotificationResponse*)response
             withCompletionHandler:(void (^)(void))completionHandler {
    NSString* subject = response.notification.request.content.userInfo[@"subject"] ?: @"";
    mmNotifyAction((char*)response.actionIdentifier.UTF8String, (char*)subject.UTF8String);
    completionHandler();
}
@end

static MMNotifyDelegate* gDelegate = nil;

int mmNotifyAvailable(void) {
    return NSBundle.mainBundle.bundleIdentifier != nil ? 1 : 0;
}

void mmNotifyInit(int requestAuth) {
    UNUserNotificationCenter* center = UNUserNotificationCenter.currentNotificationCenter;
    gDelegate = [MMNotifyDelegate new];
    center.delegate = gDelegate;

    UNNotificationAction* ignore = [UNNotificationAction actionWithIdentifier:@"ignore"
                                                                        title:@"Don't Alert for This App"
                                                                      options:UNNotificationActionOptionNone];
    UNNotificationCategory* cpu = [UNNotificationCategory categoryWithIdentifier:@"cpu"
                                                                         actions:@[ ignore ]
                                                               intentIdentifiers:@[]
                                                                         options:UNNotificationCategoryOptionNone];
    [center setNotificationCategories:[NSSet setWithObject:cpu]];

    if (requestAuth) {
        [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                              completionHandler:^(BOOL granted, NSError* err) {
            if (!granted) NSLog(@"Notifications not authorized: %@", err);
        }];
    }
}

static NSString* str(const char* s) { return [NSString stringWithUTF8String:s] ?: @""; }

void mmNotifySend(long long reqID, const char* ident, const char* title, const char* body,
                  const char* category, const char* subject) {
    // Copy the C strings now; the Go side frees them when this returns.
    NSString* identS = str(ident), *titleS = str(title), *bodyS = str(body);
    NSString* categoryS = str(category), *subjectS = str(subject);

    UNUserNotificationCenter* center = UNUserNotificationCenter.currentNotificationCenter;
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                          completionHandler:^(BOOL granted, NSError* authErr) {
        if (!granted) {
            NSString* msg = authErr ? authErr.localizedDescription
                                    : @"notifications are turned off for Mac Monitor in System Settings";
            mmNotifyDone(reqID, (char*)msg.UTF8String);
            return;
        }
        UNMutableNotificationContent* content = [UNMutableNotificationContent new];
        content.title = titleS;
        content.body = bodyS;
        content.sound = UNNotificationSound.defaultSound;
        content.categoryIdentifier = categoryS;
        content.userInfo = @{@"subject" : subjectS};
        // Reusing the alert key as identifier replaces an older notification
        // about the same thing instead of stacking them.
        UNNotificationRequest* req = [UNNotificationRequest requestWithIdentifier:identS
                                                                          content:content
                                                                          trigger:nil];
        [center addNotificationRequest:req withCompletionHandler:^(NSError* err) {
            mmNotifyDone(reqID, err ? (char*)err.localizedDescription.UTF8String : NULL);
        }];
    }];
}
