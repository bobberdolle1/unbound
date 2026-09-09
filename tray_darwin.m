#import <Cocoa/Cocoa.h>
#import "tray_darwin.h"

extern void onTrayAction(int actionTag);
extern void onTraySelectProfile(int profileIndex);
extern void onDockReopen(void);

@interface UnboundTrayDelegate : NSObject
- (void)menuAction:(id)sender;
- (void)profileAction:(id)sender;
+ (NSImage *)unboundTemplateLogoImage;
@end

@implementation UnboundTrayDelegate
- (void)menuAction:(id)sender {
    NSMenuItem *item = (NSMenuItem *)sender;
    onTrayAction((int)item.tag);
}

- (void)profileAction:(id)sender {
    NSMenuItem *item = (NSMenuItem *)sender;
    onTraySelectProfile((int)item.tag);
}

+ (NSImage *)unboundTemplateLogoImage {
    NSSize size = NSMakeSize(18, 18);
    NSImage *image = [NSImage imageWithSize:size flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
        CGContextRef ctx = [[NSGraphicsContext currentContext] CGContext];
        CGContextClearRect(ctx, dstRect);
        
        // 14x14 pt logo centered inside 18x18 pt canvas (2pt margin)
        CGFloat scale = 14.0 / 512.0;
        CGContextTranslateCTM(ctx, 2.0, 16.0);
        CGContextScaleCTM(ctx, scale, -scale);
        
        [[NSColor blackColor] setFill];
        
        // Path 1: U shape
        NSBezierPath *path = [NSBezierPath bezierPath];
        [path moveToPoint:NSMakePoint(96, 80)];
        [path lineToPoint:NSMakePoint(168, 80)];
        [path lineToPoint:NSMakePoint(168, 344)];
        [path curveToPoint:NSMakePoint(192, 368) controlPoint1:NSMakePoint(168, 357.25) controlPoint2:NSMakePoint(178.75, 368)];
        [path lineToPoint:NSMakePoint(320, 368)];
        [path curveToPoint:NSMakePoint(344, 344) controlPoint1:NSMakePoint(333.25, 368) controlPoint2:NSMakePoint(344, 357.25)];
        [path lineToPoint:NSMakePoint(344, 240)];
        [path lineToPoint:NSMakePoint(416, 240)];
        [path lineToPoint:NSMakePoint(416, 344)];
        [path curveToPoint:NSMakePoint(320, 440) controlPoint1:NSMakePoint(416, 397.02) controlPoint2:NSMakePoint(373.02, 440)];
        [path lineToPoint:NSMakePoint(192, 440)];
        [path curveToPoint:NSMakePoint(96, 344) controlPoint1:NSMakePoint(138.98, 440) controlPoint2:NSMakePoint(96, 397.02)];
        [path closePath];
        [path fill];
        
        // Path 2: Rounded rect notch
        NSRect notchRect = NSMakeRect(344, 80, 72, 112);
        NSBezierPath *notch = [NSBezierPath bezierPathWithRoundedRect:notchRect xRadius:16 yRadius:16];
        [notch fill];
        
        return YES;
    }];
    [image setTemplate:YES];
    return image;
}
@end

static NSStatusItem *globalStatusItem = nil;
static UnboundTrayDelegate *globalTrayDelegate = nil;

void setupDockClickObserver(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSNotificationCenter defaultCenter] addObserverForName:NSApplicationDidBecomeActiveNotification
                                                          object:nil
                                                           queue:[NSOperationQueue mainQueue]
                                                      usingBlock:^(NSNotification *note) {
            onDockReopen();
        }];
    });
}

void setNativeTrayIcon(const void *bytes, int length) {
    // Vector icon is rendered natively via unboundTemplateLogoImage
}

void initNativeTray(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (globalStatusItem == nil) {
            globalStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
            if (globalStatusItem.button != nil) {
                globalStatusItem.button.image = [UnboundTrayDelegate unboundTemplateLogoImage];
                globalStatusItem.button.imagePosition = NSImageOnly;
                globalStatusItem.button.title = @"";
            }
            if (globalTrayDelegate == nil) {
                globalTrayDelegate = [[UnboundTrayDelegate alloc] init];
            }
            NSMenu *menu = [[NSMenu alloc] initWithTitle:@"UNBOUND"];
            NSMenuItem *statusItem = [[NSMenuItem alloc] initWithTitle:@"Статус: Отключено" action:nil keyEquivalent:@""];
            [menu addItem:statusItem];
            
            [menu addItem:[NSMenuItem separatorItem]];
            
            NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"Показать UNBOUND" action:@selector(menuAction:) keyEquivalent:@""];
            showItem.target = globalTrayDelegate;
            showItem.tag = 1;
            [menu addItem:showItem];

            NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Завершить UNBOUND" action:@selector(menuAction:) keyEquivalent:@"q"];
            quitItem.target = globalTrayDelegate;
            quitItem.tag = 6;
            [menu addItem:quitItem];

            globalStatusItem.menu = menu;
        }
    });
}

void updateNativeTray(const char *statusText, const char *pingText, int isRunning, int activeProfileIndex, const char **profileNames, int profileCount) {
    NSString *statusStr = statusText ? [NSString stringWithUTF8String:statusText] : @"Статус: Отключено";
    NSString *pingStr = pingText ? [NSString stringWithUTF8String:pingText] : @"Пинг: —";

    NSMutableArray<NSString *> *profiles = [NSMutableArray arrayWithCapacity:profileCount];
    for (int i = 0; i < profileCount; i++) {
        if (profileNames[i]) {
            [profiles addObject:[NSString stringWithUTF8String:profileNames[i]]];
        }
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        if (globalStatusItem == nil) {
            initNativeTray();
        }
        if (globalTrayDelegate == nil) {
            globalTrayDelegate = [[UnboundTrayDelegate alloc] init];
        }

        NSMenu *menu = [[NSMenu alloc] initWithTitle:@"UNBOUND"];

        // 1. Status & Ping
        NSMenuItem *statusMenuItem = [[NSMenuItem alloc] initWithTitle:statusStr action:nil keyEquivalent:@""];
        [menu addItem:statusMenuItem];

        NSMenuItem *pingMenuItem = [[NSMenuItem alloc] initWithTitle:pingStr action:nil keyEquivalent:@""];
        [menu addItem:pingMenuItem];

        [menu addItem:[NSMenuItem separatorItem]];

        // 2. Connect / Disconnect button
        if (!isRunning) {
            NSMenuItem *connItem = [[NSMenuItem alloc] initWithTitle:@"Подключить" action:@selector(menuAction:) keyEquivalent:@"r"];
            connItem.target = globalTrayDelegate;
            connItem.tag = 3;
            [menu addItem:connItem];
        } else {
            NSMenuItem *disconnItem = [[NSMenuItem alloc] initWithTitle:@"Отключить" action:@selector(menuAction:) keyEquivalent:@"t"];
            disconnItem.target = globalTrayDelegate;
            disconnItem.tag = 4;
            [menu addItem:disconnItem];
        }

        NSMenuItem *autoTuneItem = [[NSMenuItem alloc] initWithTitle:@"Автоподбор" action:@selector(menuAction:) keyEquivalent:@""];
        autoTuneItem.target = globalTrayDelegate;
        autoTuneItem.tag = 5;
        [menu addItem:autoTuneItem];

        [menu addItem:[NSMenuItem separatorItem]];

        // 3. Profiles Submenu
        if (profiles.count > 0) {
            NSMenuItem *profParentItem = [[NSMenuItem alloc] initWithTitle:@"Профили" action:nil keyEquivalent:@""];
            NSMenu *profSubMenu = [[NSMenu alloc] initWithTitle:@"Профили"];
            for (int i = 0; i < (int)profiles.count; i++) {
                NSString *title = profiles[i];
                if (isRunning && i == activeProfileIndex) {
                    title = [NSString stringWithFormat:@"✓ %@", profiles[i]];
                }
                NSMenuItem *subItem = [[NSMenuItem alloc] initWithTitle:title action:@selector(profileAction:) keyEquivalent:@""];
                subItem.target = globalTrayDelegate;
                subItem.tag = i;
                [profSubMenu addItem:subItem];
            }
            [profParentItem setSubmenu:profSubMenu];
            [menu addItem:profParentItem];
            [menu addItem:[NSMenuItem separatorItem]];
        }

        // 4. Window controls
        NSMenuItem *showWinItem = [[NSMenuItem alloc] initWithTitle:@"Показать главное окно" action:@selector(menuAction:) keyEquivalent:@""];
        showWinItem.target = globalTrayDelegate;
        showWinItem.tag = 1;
        [menu addItem:showWinItem];

        NSMenuItem *hideWinItem = [[NSMenuItem alloc] initWithTitle:@"Скрыть окно" action:@selector(menuAction:) keyEquivalent:@"h"];
        hideWinItem.target = globalTrayDelegate;
        hideWinItem.tag = 2;
        [menu addItem:hideWinItem];

        [menu addItem:[NSMenuItem separatorItem]];

        // 5. Quit
        NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Завершить UNBOUND" action:@selector(menuAction:) keyEquivalent:@"q"];
        quitItem.target = globalTrayDelegate;
        quitItem.tag = 6;
        [menu addItem:quitItem];

        globalStatusItem.menu = menu;

        if (globalStatusItem.button != nil) {
            if (globalStatusItem.button.image == nil) {
                globalStatusItem.button.image = [UnboundTrayDelegate unboundTemplateLogoImage];
            }
            globalStatusItem.button.imagePosition = NSImageOnly;
            globalStatusItem.button.title = @"";
            globalStatusItem.button.toolTip = [NSString stringWithFormat:@"UNBOUND — %@", statusStr];
        }
    });
}
