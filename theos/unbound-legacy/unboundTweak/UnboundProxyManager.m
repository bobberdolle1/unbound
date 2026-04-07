/* Unbound Legacy -- Tweak Proxy Manager (non-ARC, substrate-compatible) */
#import <Foundation/Foundation.h>
#import <SystemConfiguration/SystemConfiguration.h>

@interface UnboundProxyManager : NSObject
+ (instancetype)sharedManager;
- (void)setSystemSOCKSProxyEnabled:(BOOL)enabled port:(NSInteger)port;
- (BOOL)isProxyActive;
@end

@implementation UnboundProxyManager
+ (instancetype)sharedManager { static UnboundProxyManager *s; static dispatch_once_t t; dispatch_once(&t,^{s=[[UnboundProxyManager alloc]init];}); return s; }
- (void)setSystemSOCKSProxyEnabled:(BOOL)enabled port:(NSInteger)port {
    SCPreferencesRef ref=SCPreferencesCreate(NULL,CFSTR("com.unbound.tweak"),NULL); if(!ref)return; SCPreferencesLock(ref,true);
    CFStringRef path=SCPreferencesGetValue(ref,kSCPrefCurrentSet);
    if(path){
        NSDictionary *set=(__bridge_transfer NSDictionary*)SCPreferencesPathGetValue(ref,path);
        if(set){
            NSDictionary *svcs=(__bridge_transfer NSDictionary*)SCPreferencesGetValue(ref,kSCPrefNetworkServices);
            if(svcs){
                CFDataRef d=CFPropertyListCreateData(NULL,(__bridge CFPropertyListRef)svcs,kCFPropertyListBinaryFormat_v1_0,0,NULL);
                NSMutableDictionary *ms=(__bridge_transfer NSMutableDictionary*)CFPropertyListCreateWithData(NULL,d,kCFPropertyListMutableContainersAndLeaves,NULL,NULL);
                if(d)CFRelease(d);
                NSDictionary *cs=set[(__bridge NSString*)kSCCompNetwork][(__bridge NSString*)kSCCompService];
                for(NSString *k in cs){NSDictionary *svc=svcs[k];if([svc[(__bridge NSString*)kSCPropUserDefinedName] isEqualToString:@"Wi-Fi"]){
                    NSMutableDictionary *px=ms[k][(__bridge NSString*)kSCEntNetProxies];
                    if(!px){px=[NSMutableDictionary dictionary];ms[k][(__bridge NSString*)kSCEntNetProxies]=px;}
                    if(enabled){px[(__bridge NSString*)kSCPropNetProxiesSOCKSEnable]=@1;px[(__bridge NSString*)kSCPropNetProxiesSOCKSProxy]=@"127.0.0.1";px[(__bridge NSString*)kSCPropNetProxiesSOCKSPort]=@(port);}
                    else{px[(__bridge NSString*)kSCPropNetProxiesSOCKSEnable]=@0;}
                    break;}}
                SCPreferencesSetValue(ref,kSCPrefNetworkServices,(__bridge CFPropertyListRef)ms);SCPreferencesCommitChanges(ref);SCPreferencesApplyChanges(ref);
            }
        }
    }
    SCPreferencesUnlock(ref);CFRelease(ref);
}
- (BOOL)isProxyActive {
    SCPreferencesRef ref=SCPreferencesCreate(NULL,CFSTR("com.unbound.check"),NULL); if(!ref)return NO;
    NSDictionary *svcs=(__bridge_transfer NSDictionary*)SCPreferencesGetValue(ref,kSCPrefNetworkServices); CFRelease(ref);
    if(!svcs)return NO;
    for(NSString *k in svcs){NSDictionary *px=svcs[k][(__bridge NSString*)kSCEntNetProxies];if(px&&([px[(__bridge NSString*)kSCPropNetProxiesSOCKSEnable]boolValue]||[px[(__bridge NSString*)kSCPropNetProxiesHTTPEnable]boolValue]))return YES;}
    return NO;
}
@end
