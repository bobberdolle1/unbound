#import "UnboundProxyManager.h"
#include <SystemConfiguration/SystemConfiguration.h>
#include <unistd.h>
#include <signal.h>

#define UNBOUND_TPWS_PATH @"/usr/local/bin/unbound-tpws"
#define UNBOUND_PID_FILE  @"/var/run/unbound-tpws.pid"

@interface UnboundProxyManager ()
@property (nonatomic, assign) BOOL isRunning;
@property (nonatomic, assign) NSInteger currentPort;
@end

@implementation UnboundProxyManager

+ (instancetype)sharedManager {
    static UnboundProxyManager *s; static dispatch_once_t t;
    dispatch_once(&t, ^{ s = [[UnboundProxyManager alloc] init]; });
    return s;
}
- (instancetype)init { self=[super init]; if(self){_isRunning=NO;_currentPort=1993;} return self; }

- (void)startEngineWithPort:(NSInteger)port strategy:(NSInteger)strategy completion:(void(^)(BOOL,NSString*))completion {
    _currentPort = port > 0 ? port : 1993;
    NSFileManager *fm = [NSFileManager defaultManager];
    if (![fm fileExistsAtPath:@"/etc/unbound"]) [fm createDirectoryAtPath:@"/etc/unbound" withIntermediateDirectories:YES attributes:nil error:nil];

    /* Kill existing */
    pid_t old = [self readPIDFile];
    if (old > 0) kill(old, SIGKILL);
    system("killall unbound-tpws 2>/dev/null");
    usleep(200000);

    char *argv[] = {strdup([UNBOUND_TPWS_PATH UTF8String]),strdup("--port"),strdup([[NSString stringWithFormat:@"%ld",(long)_currentPort] UTF8String]),NULL};
    pid_t pid;
    int r = posix_spawn(&pid, [UNBOUND_TPWS_PATH UTF8String], NULL, NULL, argv, NULL);
    for(int i=0;argv[i];i++) free(argv[i]);

    if (r != 0) { if(completion)completion(NO,[NSString stringWithFormat:@"spawn failed: %s",strerror(r)]); return; }

    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,1*NSEC_PER_SEC),dispatch_get_main_queue(),^{
        BOOL running = kill(pid,0)==0;
        if(running){
            _isRunning=YES;
            [self setSystemProxyEnabled:YES port:_currentPort];
            if(completion)completion(YES,[NSString stringWithFormat:@"Engine running on port %ld",(long)_currentPort]);
        } else {
            if(completion)completion(NO,@"Engine died immediately");
        }
    });
}

- (void)stopEngineWithCompletion:(void(^)(BOOL))completion {
    [self setSystemProxyEnabled:NO port:0];
    pid_t pid = [self readPIDFile];
    BOOL ok = NO;
    if(pid>0 && kill(pid,SIGTERM)==0){
        for(int i=0;i<10;i++){usleep(100000);if(kill(pid,0)!=0){ok=YES;break;}}
        if(!ok){kill(pid,SIGKILL);usleep(200000);ok=kill(pid,0)!=0;}
    } else {
        system("killall unbound-tpws 2>/dev/null"); usleep(500000);
    }
    _isRunning=!ok;
    if(completion)completion(ok);
}

- (void)getEngineStatusWithCompletion:(void(^)(BOOL,NSInteger,NSString*))completion {
    BOOL running = kill([self readPIDFile],0)==0;
    NSString *msg = running ? [NSString stringWithFormat:@"Proxy 127.0.0.1:%ld active",(long)_currentPort] : @"Engine inactive";
    if(completion)completion(running, running?_currentPort:0, msg);
}

- (void)testConnectionWithCompletion:(void(^)(BOOL,NSString*))completion {
    NSURLSession *session = [NSURLSession sharedSession];
    [[session dataTaskWithURL:[NSURL URLWithString:@"https://www.google.com"] completionHandler:^(NSData *data, NSURLResponse *response, NSError *error) {
        dispatch_async(dispatch_get_main_queue(), ^{
            if(error){if(completion)completion(NO,error.localizedDescription);}
            else{NSHTTPURLResponse *h=(NSHTTPURLResponse*)response; if(completion)completion(YES,[NSString stringWithFormat:@"HTTP %ld -- bypass working",(long)h.statusCode]);}
        });
    }] resume];
}

/* ---- SCPreferences proxy injection ---- */
- (void)setSystemProxyEnabled:(BOOL)enabled port:(NSInteger)port {
    SCPreferencesRef ref = SCPreferencesCreate(NULL, CFSTR("com.unbound.legacy"), NULL);
    if(!ref)return;
    SCPreferencesLock(ref, true);
    CFStringRef path = SCPreferencesGetValue(ref, kSCPrefCurrentSet);
    if(path){
        NSDictionary *set = (__bridge NSDictionary*)SCPreferencesPathGetValue(ref, path);
        if(set){
            NSDictionary *svcs = (__bridge NSDictionary*)SCPreferencesGetValue(ref, kSCPrefNetworkServices);
            if(svcs){
                NSData *d = [NSPropertyListSerialization dataWithPropertyList:svcs format:NSPropertyListBinaryFormat_v1_0 options:0 error:nil];
                NSMutableDictionary *ms = [NSPropertyListSerialization propertyListWithData:d options:NSPropertyListMutableContainersAndLeaves format:NULL error:nil];
                NSDictionary *cs = set[(__bridge NSString*)kSCCompNetwork][(__bridge NSString*)kSCCompService];
                for(NSString *k in cs){
                    NSDictionary *svc = svcs[k];
                    if([svc[(__bridge NSString*)kSCPropUserDefinedName] isEqualToString:@"Wi-Fi"]){
                        NSMutableDictionary *px = ms[k][(__bridge NSString*)kSCEntNetProxies];
                        if(!px){px=[NSMutableDictionary dictionary]; ms[k][(__bridge NSString*)kSCEntNetProxies]=px;}
                        if(enabled){
                            px[(__bridge NSString*)kSCPropNetProxiesSOCKSEnable]=@1;
                            px[(__bridge NSString*)kSCPropNetProxiesSOCKSProxy]=@"127.0.0.1";
                            px[(__bridge NSString*)kSCPropNetProxiesSOCKSPort]=@(port);
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPEnable]=@1;
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPProxy]=@"127.0.0.1";
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPPort]=@(port);
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPSEnable]=@1;
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPSProxy]=@"127.0.0.1";
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPSPort]=@(port);
                        } else {
                            px[(__bridge NSString*)kSCPropNetProxiesSOCKSEnable]=@0;
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPEnable]=@0;
                            px[(__bridge NSString*)kSCPropNetProxiesHTTPSEnable]=@0;
                        }
                        break;
                    }
                }
                SCPreferencesSetValue(ref, kSCPrefNetworkServices, (__bridge CFPropertyListRef)ms);
                SCPreferencesCommitChanges(ref);
                SCPreferencesApplyChanges(ref);
            }
        }
    }
    SCPreferencesUnlock(ref);
    CFRelease(ref);
}

- (pid_t)readPIDFile {
    NSString *s = [NSString stringWithContentsOfFile:UNBOUND_PID_FILE encoding:NSASCIIStringEncoding error:nil];
    return s ? (pid_t)atoi([s UTF8String]) : 0;
}

@end
