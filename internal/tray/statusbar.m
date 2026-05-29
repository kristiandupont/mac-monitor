#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#include "statusbar.h"

// Declared in exports.go via //export.
extern void onMenuItemClicked(int itemID);

static NSStatusItem* gItem   = nil;
static NSMenu*       gMenu   = nil;
static NSImage**     gImages = nil;
static int           gImageCount = 0;

@interface MenuTarget : NSObject
@end
@implementation MenuTarget
- (void)menuItemClicked:(id)sender {
    onMenuItemClicked((int)((NSMenuItem*)sender).tag);
}
@end
static MenuTarget* gTarget = nil;

void initCocoaApp(void) {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
}

void setupStatusItem(const char* tooltip) {
    gTarget = [[MenuTarget alloc] init];

    gItem = [[NSStatusBar systemStatusBar]
        statusItemWithLength:NSSquareStatusItemLength];
    gItem.button.toolTip = [NSString stringWithUTF8String:tooltip];
    gItem.button.imageScaling = NSImageScaleProportionallyDown;

    gMenu = [[NSMenu alloc] initWithTitle:@""];
    [gMenu setAutoenablesItems:NO];
    gItem.menu = gMenu;
}

void runCocoaApp(void) {
    [NSApp run];
}

void quitCocoaApp(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSStatusBar systemStatusBar] removeStatusItem:gItem];
        [NSApp terminate:nil];
    });
}

void preloadImagesInit(int count) {
    gImages = (NSImage* __strong*)calloc(count, sizeof(NSImage*));
    gImageCount = count;
}

void loadImageAtIndex(int idx, const unsigned char* data, int len) {
    if (idx < 0 || idx >= gImageCount) return;
    NSData*  nsdata = [NSData dataWithBytes:data length:len];
    NSImage* img    = [[NSImage alloc] initWithData:nsdata];
    // Declare logical size as 22pt so macOS treats 44px as @2x (retina).
    [img setSize:NSMakeSize(22, 22)];
    gImages[idx] = img;
}

void setIconIndex(int idx) {
    if (idx < 0 || idx >= gImageCount || !gImages[idx]) return;
    NSImage* img = gImages[idx];
    dispatch_async(dispatch_get_main_queue(), ^{
        // Disable implicit CA cross-fade animation. Without this, each setImage:
        // call starts a ~100ms transition that keeps the layer dirty at 60Hz
        // for its entire duration — causing continuous full-refresh-rate redraws
        // even when the icon only changes at ~10fps.
        [CATransaction begin];
        [CATransaction setDisableActions:YES];
        gItem.button.image = img;
        [CATransaction commit];
    });
}

void addMenuItemCStr(const char* title, int itemID) {
    NSMenuItem* item = [[NSMenuItem alloc]
        initWithTitle:[NSString stringWithUTF8String:title]
        action:@selector(menuItemClicked:)
        keyEquivalent:@""];
    item.target  = gTarget;
    item.tag     = itemID;
    item.enabled = YES;
    [gMenu addItem:item];
}

void addMenuSeparatorItem(void) {
    [gMenu addItem:[NSMenuItem separatorItem]];
}
