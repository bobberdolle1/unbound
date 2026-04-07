#import <Foundation/Foundation.h>
@interface UnboundProxyManager : NSObject
+ (instancetype)sharedManager;
- (void)startEngineWithPort:(NSInteger)port strategy:(NSInteger)strategy completion:(void(^)(BOOL success, NSString *message))completion;
- (void)stopEngineWithCompletion:(void(^)(BOOL success))completion;
- (void)getEngineStatusWithCompletion:(void(^)(BOOL running, NSInteger port, NSString *message))completion;
- (void)testConnectionWithCompletion:(void(^)(BOOL success, NSString *message))completion;
@end
