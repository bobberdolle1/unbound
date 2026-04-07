#import <UIKit/UIKit.h>
#import "UnboundAppDelegate.h"
#import "UnboundSkeuomorphicViewController.h"
#import "UnboundModernViewController.h"

@implementation UnboundAppDelegate

- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
    self.window = [[UIWindow alloc] initWithFrame:[[UIScreen mainScreen] bounds]];
    UIViewController *rootVC;
    float sysVersion = [[[UIDevice currentDevice] systemVersion] floatValue];
    if (sysVersion < 7.0) {
        rootVC = [[UnboundSkeuomorphicViewController alloc] init];
    } else {
        rootVC = [[UnboundModernViewController alloc] init];
    }
    self.window.rootViewController = rootVC;
    [self.window makeKeyAndVisible];
    return YES;
}

@end
