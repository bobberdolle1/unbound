#import "UnboundLinenBackgroundView.h"
@implementation UnboundLinenBackgroundView
- (instancetype)initWithFrame:(CGRect)frame { self=[super initWithFrame:frame]; if(self){self.backgroundColor=[UIColor clearColor];self.opaque=NO;} return self; }
- (void)drawRect:(CGRect)rect {
    CGContextRef ctx = UIGraphicsGetCurrentContext();
    CGSize sz = self.bounds.size;
    CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
    CGFloat loc[]={0,1}, comp[]={0.60,0.58,0.55,1, 0.48,0.46,0.44,1};
    CGGradientRef g = CGGradientCreateWithColorComponents(cs,comp,loc,2);
    CGContextDrawLinearGradient(ctx,g,CGPointZero,CGPointMake(0,sz.height),0);
    CGGradientRelease(g);
    CGContextSetAlpha(ctx,0.06);
    for(CGFloat y=0;y<sz.height;y+=2){
        CGContextSetStrokeColorWithColor(ctx,(fmod(y,4)<2)?[UIColor whiteColor].CGColor:[UIColor blackColor].CGColor);
        CGContextSetLineWidth(ctx,0.5);CGContextBeginPath(ctx);CGContextMoveToPoint(ctx,0,y);CGContextAddLineToPoint(ctx,sz.width,y);CGContextStrokePath(ctx);
    }
    for(CGFloat x=0;x<sz.width;x+=3){
        CGContextSetStrokeColorWithColor(ctx,(fmod(x,6)<3)?[UIColor whiteColor].CGColor:[UIColor blackColor].CGColor);
        CGContextSetLineWidth(ctx,0.5);CGContextBeginPath(ctx);CGContextMoveToPoint(ctx,x,0);CGContextAddLineToPoint(ctx,x,sz.height);CGContextStrokePath(ctx);
    }
    CGContextSetAlpha(ctx,0.03);
    for(int i=0;i<3000;i++){CGFloat x=arc4random_uniform((uint32_t)sz.width),y=arc4random_uniform((uint32_t)sz.height),b=(arc4random_uniform(100)/100.0);CGFloat c[]={b,b,b,1};CGColorRef col=CGColorCreate(cs,c);CGContextSetFillColorWithColor(ctx,col);CGContextFillRect(ctx,CGRectMake(x,y,1,1));CGColorRelease(col);}
    CGColorSpaceRelease(cs);
}
@end
