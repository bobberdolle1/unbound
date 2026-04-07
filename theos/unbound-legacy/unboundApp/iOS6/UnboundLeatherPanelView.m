#import "UnboundLeatherPanelView.h"
@implementation UnboundLeatherPanelView
- (instancetype)initWithFrame:(CGRect)frame {
    self=[super initWithFrame:frame];
    if(self){self.backgroundColor=[UIColor clearColor];self.layer.cornerRadius=10;self.layer.masksToBounds=YES;
        self.layer.shadowColor=[UIColor blackColor].CGColor;self.layer.shadowOffset=CGSizeMake(0,4);self.layer.shadowOpacity=0.4;self.layer.shadowRadius=6;}
    return self;
}
- (void)drawRect:(CGRect)rect {
    CGContextRef ctx=UIGraphicsGetCurrentContext(); CGSize sz=self.bounds.size;
    CGColorSpaceRef cs=CGColorSpaceCreateDeviceRGB();
    CGFloat loc[]={0,0.02,0.5,0.98,1}, comp[]={0.38,0.30,0.24,1, 0.28,0.22,0.17,1, 0.22,0.17,0.13,1, 0.28,0.22,0.17,1, 0.18,0.14,0.10,1};
    CGGradientRef g=CGGradientCreateWithColorComponents(cs,comp,loc,2);
    CGContextDrawLinearGradient(ctx,g,CGPointZero,CGPointMake(0,sz.height),0);CGGradientRelease(g);
    CGContextSetAlpha(ctx,0.08);
    for(int i=0;i<5000;i++){CGFloat x=arc4random_uniform((uint32_t)sz.width),y=arc4random_uniform((uint32_t)sz.height),b=0.3+(arc4random_uniform(40)/100.0);
        CGFloat c[]={b,b*0.85,b*0.7,1};CGColorRef col=CGColorCreate(cs,c);CGContextSetFillColorWithColor(ctx,col);
        CGContextFillEllipseInRect(ctx,CGRectMake(x,y,1+arc4random_uniform(2),1+arc4random_uniform(2)));CGColorRelease(col);}
    CGContextSetAlpha(ctx,1.0);
    /* Stitching */
    CGContextSetStrokeColorWithColor(ctx,[UIColor colorWithWhite:0.75 alpha:0.6].CGColor);
    CGContextSetLineWidth(ctx,1.5);CGContextSetLineDash(ctx,0,(CGFloat[]){6,4},2);
    CGFloat ins=5,r=6;
    CGContextBeginPath(ctx);
    CGContextMoveToPoint(ctx,ins+r,ins);CGContextAddLineToPoint(ctx,sz.width-ins-r,ins);
    CGContextAddArcToPoint(ctx,sz.width-ins,ins,sz.width-ins,ins+r,r);
    CGContextAddLineToPoint(ctx,sz.width-ins,sz.height-ins-r);
    CGContextAddArcToPoint(ctx,sz.width-ins,sz.height-ins,sz.width-ins-r,sz.height-ins,r);
    CGContextAddLineToPoint(ctx,ins+r,sz.height-ins);
    CGContextAddArcToPoint(ctx,ins,sz.height-ins,ins,sz.height-ins-r,r);
    CGContextAddLineToPoint(ctx,ins,ins+r);
    CGContextAddArcToPoint(ctx,ins,ins,ins+r,ins,r);
    CGContextClosePath(ctx);CGContextStrokePath(ctx);CGContextSetLineDash(ctx,0,NULL,0);
    /* Gloss overlay */
    CGContextSaveGState(ctx);CGContextBeginPath(ctx);
    CGContextMoveToPoint(ctx,r+1,1);CGContextAddLineToPoint(ctx,sz.width-r-1,1);
    CGContextAddArcToPoint(ctx,sz.width-1,1,sz.width-1,r+1,r);
    CGContextAddLineToPoint(ctx,sz.width-1,sz.height*0.35);CGContextAddLineToPoint(ctx,1,sz.height*0.35);
    CGContextAddLineToPoint(ctx,1,r+1);CGContextAddArcToPoint(ctx,1,1,r+1,1,r);
    CGContextClosePath(ctx);CGContextClip(ctx);
    CGColorSpaceRef cs2=CGColorSpaceCreateDeviceRGB();
    CGFloat gl[]={0,1},gc[]={1,1,1,0.10, 1,1,1,0};
    CGGradientRef gg=CGGradientCreateWithColorComponents(cs2,gc,gl,2);
    CGContextDrawLinearGradient(ctx,gg,CGPointMake(sz.width/2,0),CGPointMake(sz.width/2,sz.height*0.35),0);
    CGGradientRelease(gg);CGColorSpaceRelease(cs2);CGContextRestoreGState(ctx);
    /* Border */
    CGContextSetStrokeColorWithColor(ctx,[UIColor colorWithWhite:0.15 alpha:0.8].CGColor);
    CGContextSetLineWidth(ctx,1.5);CGContextBeginPath(ctx);
    CGContextMoveToPoint(ctx,r+0.75,0.75);CGContextAddLineToPoint(ctx,sz.width-r-0.75,0.75);
    CGContextAddArcToPoint(ctx,sz.width-0.75,0.75,sz.width-0.75,r+0.75,r);
    CGContextAddLineToPoint(ctx,sz.width-0.75,sz.height-r-0.75);
    CGContextAddArcToPoint(ctx,sz.width-0.75,sz.height-0.75,sz.width-r-0.75,sz.height-0.75,r);
    CGContextAddLineToPoint(ctx,r+0.75,sz.height-0.75);
    CGContextAddArcToPoint(ctx,0.75,sz.height-0.75,0.75,sz.height-r-0.75,r);
    CGContextAddLineToPoint(ctx,0.75,r+0.75);
    CGContextAddArcToPoint(ctx,0.75,0.75,r+0.75,0.75,r);
    CGContextClosePath(ctx);CGContextStrokePath(ctx);
    CGColorSpaceRelease(cs);
}
@end
