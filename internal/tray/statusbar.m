#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#include "statusbar.h"

extern void onMenuItemClicked(int itemID);

static NSStatusItem* gItem            = nil;
static NSMenu*       gMenu            = nil;
static NSView*       gIconView        = nil;
static CALayer*      gRotateLayer     = nil;
static NSImage**     gColorImages     = nil;
static int           gColorCount      = 0;
static int           gCurrentColorIdx = -1;

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
    gItem.button.wantsLayer = YES;

    // Layer-backed subview for the fan icon. We rotate a dedicated CALayer sublayer
    // (gRotateLayer) rather than the NSView-backed layer itself. NSView-backed layers
    // have anchorPoint={0,0} (AppKit manages position from the origin), so setting
    // transform on them rotates around the corner. The sublayer has full control over
    // its anchorPoint={0.5,0.5} so rotation is always around the icon's center.
    gIconView = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 22, 22)];
    gIconView.wantsLayer = YES;
    gIconView.translatesAutoresizingMaskIntoConstraints = NO;
    [gItem.button addSubview:gIconView];
    [NSLayoutConstraint activateConstraints:@[
        [gIconView.centerXAnchor constraintEqualToAnchor:gItem.button.centerXAnchor],
        [gIconView.centerYAnchor constraintEqualToAnchor:gItem.button.centerYAnchor],
        [gIconView.widthAnchor constraintEqualToConstant:22],
        [gIconView.heightAnchor constraintEqualToConstant:22],
    ]];

    gRotateLayer = [CALayer layer];
    gRotateLayer.bounds = CGRectMake(0, 0, 22, 22);
    gRotateLayer.anchorPoint = CGPointMake(0.5, 0.5);
    gRotateLayer.position = CGPointMake(11, 11);
    gRotateLayer.contentsGravity = kCAGravityResizeAspect;
    gRotateLayer.contentsScale = [NSScreen mainScreen].backingScaleFactor;
    [gIconView.layer addSublayer:gRotateLayer];

    gMenu = [[NSMenu alloc] initWithTitle:@""];
    [gMenu setAutoenablesItems:NO];
    gItem.menu = gMenu;
}

void runCocoaApp(void) { [NSApp run]; }

void quitCocoaApp(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSStatusBar systemStatusBar] removeStatusItem:gItem];
        [NSApp terminate:nil];
    });
}

void preloadColorImagesInit(int count) {
    gColorImages = (NSImage* __strong*)calloc(count, sizeof(NSImage*));
    gColorCount = count;
}

void loadColorImage(int idx, const unsigned char* data, int len) {
    if (idx < 0 || idx >= gColorCount) return;
    NSData*  nsdata = [NSData dataWithBytes:data length:len];
    NSImage* img    = [[NSImage alloc] initWithData:nsdata];
    [img setSize:NSMakeSize(22, 22)];
    gColorImages[idx] = img;
}

void setIconFrame(int colorIdx, float angleDeg) {
    if (colorIdx < 0 || colorIdx >= gColorCount || !gColorImages[colorIdx]) return;
    int   capturedIdx   = colorIdx;
    float capturedAngle = angleDeg;
    dispatch_async(dispatch_get_main_queue(), ^{
        [CATransaction begin];
        [CATransaction setDisableActions:YES];
        if (capturedIdx != gCurrentColorIdx) {
            gRotateLayer.contents = gColorImages[capturedIdx];
            gCurrentColorIdx = capturedIdx;
        }
        float rad = capturedAngle * (float)M_PI / 180.0f;
        gRotateLayer.transform = CATransform3DMakeRotation(rad, 0, 0, 1);
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
