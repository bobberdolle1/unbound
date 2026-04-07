/* Unbound Legacy -- Cydia Substrate / ElleKit Tweak */
#import <UIKit/UIKit.h>
#import <Foundation/Foundation.h>
#import <SystemConfiguration/SystemConfiguration.h>
#import <substrate.h>

static BOOL g_enabled = NO;
static NSInteger g_port = 1993;
#define TPWS_PATH "/usr/local/bin/unbound-tpws"
#define PID_FILE  "/var/run/unbound-tpws.pid"

static void SetProxy(BOOL on, NSInteger port) {
    SCPreferencesRef ref = SCPreferencesCreate(NULL, CFSTR("com.unbound.tweak"), NULL);
    if(!ref)return; SCPreferencesLock(ref, true);
    CFStringRef path = SCPreferencesGetValue(ref, kSCPrefCurrentSet);
    if(path){
        NSDictionary *set = (__bridge NSDictionary*)SCPreferencesPathGetValue(ref, path);
        if(set){
            NSDictionary *svcs = (__bridge NSDictionary*)SCPreferencesGetValue(ref, kSCPrefNetworkServices);
            if(svcs){
                NSData *d=[NSPropertyListSerialization dataWithPropertyList:svcs format:NSPropertyListBinaryFormat_v1_0 options:0 error:nil];
                NSMutableDictionary *ms=[NSPropertyListSerialization propertyListWithData:d options:NSPropertyListMutableContainersAndLeaves format:NULL error:nil];
                NSDictionary *cs=set[(__bridge NSString*)kSCCompNetwork][(__bridge NSString*)kSCCompService];
                for(NSString *k in cs){
                    NSDictionary *svc=svcs[k];
                    if([svc[(__bridge NSString*)kSCPropUserDefinedName] isEqualToString:@"Wi-Fi"]){
                        NSMutableDictionary *px=ms[k][(__bridge NSString*)kSCEntNetProxies];
                        if(!px){px=[NSMutableDictionary dictionary];ms[k][(__bridge NSString*)kSCEntNetProxies]=px;}
                        if(on){px[(__bridge NSString*)kSCPropNetProxiesSOCKSEnable]=@1;px[(__bridge NSString*)kSCPropNetProxiesSOCKSProxy]=@"127.0.0.1";px[(__bridge NSString*)kSCPropNetProxiesSOCKSPort]=@(port);
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPEnable]=@1;px[(__bridge NSString*)kSCPropNetProxiesHTTPProxy]=@"127.0.0.1";px[(__bridge NSString*)kSCPropNetProxiesHTTPPort]=@(port);
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPSEnable]=@1;px[(__bridge NSString*)kSCPropNetProxiesHTTPSProxy]=@"127.0.0.1";px[(__bridge NSString*)kSCPropNetProxiesHTTPSPort]=@(port);
                        } else {px[(__bridge NSString*)kSCPropNetProxiesSOCKSEnable]=@0;px[(__bridge NSString*)kSCPropNetProxiesHTTPEnable]=@0;px[(__bridge NSString*)kSCPropNetProxiesHTTPSEnable]=@0;}
                        break;}}
                SCPreferencesSetValue(ref,kSCPrefNetworkServices,(__bridge CFPropertyListRef)ms);SCPreferencesCommitChanges(ref);SCPreferencesApplyChanges(ref);
            }
        }
    }
    SCPreferencesUnlock(ref);CFRelease(ref);
}

static pid_t ReadPID(void) { NSString *s=[NSString stringWithContentsOfFile:[NSString stringWithUTF8String:PID_FILE] encoding:NSASCIIStringEncoding error:nil]; return s?(pid_t)atoi([s UTF8String]):0; }
static BOOL IsRunning(void) { pid_t p=ReadPID(); return p>0&&kill(p,0)==0; }

static void StartDaemon(NSInteger port) {
    if(IsRunning())kill(ReadPID(),SIGKILL); usleep(200000);
    char ps[16];snprintf(ps,sizeof(ps),"%ld",(long)port);
    char *av[]={strdup(TPWS_PATH),strdup("--port"),strdup(ps),NULL};
    pid_t pid;posix_spawn(&pid,TPWS_PATH,NULL,NULL,av,NULL);
    for(int i=0;av[i];i++)free(av[i]);
    g_port=port;g_enabled=YES;SetProxy(YES,port);
}
static void StopDaemon(void) {
    pid_t p=ReadPID(); if(p>0){kill(p,SIGTERM);usleep(500000);if(kill(p,0)==0)kill(p,SIGKILL);}
    system("killall unbound-tpws 2>/dev/null"); g_enabled=NO;SetProxy(NO,0);
}

%hook SpringBoard
- (void)applicationDidFinishLaunching:(id)application {
    %orig;
    [[NSNotificationCenter defaultCenter] addObserverForName:@"com.unbound.toggle" object:nil queue:[NSOperationQueue mainQueue] usingBlock:^(NSNotification *n){
        BOOL en=[n.userInfo[@"enabled"] boolValue]; NSInteger p=[n.userInfo[@"port"] integerValue]?:1993;
        if(en&&!g_enabled)StartDaemon(p); else if(!en&&g_enabled)StopDaemon();
        [[NSNotificationCenter defaultCenter] postNotificationName:@"com.unbound.status" object:nil userInfo:@{@"running":@(g_enabled),@"port":@(g_port)}];
    }];
}
%end

%ctor {
    %init;
    if(IsRunning()){g_enabled=YES;g_port=1993;SetProxy(YES,g_port);}
}
