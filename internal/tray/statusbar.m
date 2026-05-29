#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#include "statusbar.h"

extern void onMenuItemClicked(int itemID);

static NSStatusItem* gItem        = nil;
static NSMenu*       gMenu        = nil;
static NSView*       gIconView    = nil;
static CALayer*      gRotateLayer = nil;
static CALayer*      gMaskLayer   = nil;

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
    // (gRotateLayer) rather than the NSView-backed layer itself — NSView-backed layers
    // have anchorPoint={0,0} so rotation would be around the corner instead of center.
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
    [gIconView.layer addSublayer:gRotateLayer];

    // The fan image's alpha channel acts as a mask over backgroundColor.
    // Tinting is then just updating backgroundColor — a free GPU property change.
    gMaskLayer = [CALayer layer];
    gMaskLayer.frame = gRotateLayer.bounds;
    gMaskLayer.contentsGravity = kCAGravityResizeAspect;
    gMaskLayer.contentsScale = [NSScreen mainScreen].backingScaleFactor;
    gRotateLayer.mask = gMaskLayer;

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

void loadBaseImage(const unsigned char* data, int len) {
    NSData*  nsdata = [NSData dataWithBytes:data length:len];
    NSImage* img    = [[NSImage alloc] initWithData:nsdata];
    [img setSize:NSMakeSize(22, 22)];
    gMaskLayer.contents = img;
}

void setIconFrame(float angleDeg, float r, float g, float b) {
    float capturedAngle = angleDeg;
    float cr = r, cg = g, cb = b;
    dispatch_async(dispatch_get_main_queue(), ^{
        [CATransaction begin];
        [CATransaction setDisableActions:YES];
        NSColor* tint = [NSColor colorWithSRGBRed:cr green:cg blue:cb alpha:1.0];
        gRotateLayer.backgroundColor = tint.CGColor;
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
