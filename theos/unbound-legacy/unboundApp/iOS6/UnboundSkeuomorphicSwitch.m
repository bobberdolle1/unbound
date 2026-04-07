#import "UnboundSkeuomorphicSwitch.h"
@implementation UnboundSkeuomorphicSwitch { BOOL _isOn; UIImageView *_knob; UILabel *_onLbl; UILabel *_offLbl; }
@synthesize isOn=_isOn;
- (instancetype)initWithFrame:(CGRect)frame {
    self=[super initWithFrame:frame]; if(self){_isOn=NO;self.backgroundColor=[UIColor clearColor];[self setup];} return self;
}
- (void)setup {
    CGSize sz=self.bounds.size;
    /* Background */
    UIGraphicsBeginImageContextWithOptions(sz,NO,1);CGContextRef ctx=UIGraphicsGetCurrentContext();
    CGColorSpaceRef cs=CGColorSpaceCreateDeviceRGB();
    CGFloat loc[]={0,1};
    CGFloat grc[]={0.15,0.70,0.10,1, 0.10,0.55,0.05,1};
    CGGradientRef gg=CGGradientCreateWithColorComponents(cs,grc,loc,2);
    CGContextSaveGState(ctx);CGContextAddRect(ctx,CGRectMake(0,0,sz.width/2,sz.height));CGContextClip(ctx);
    CGContextDrawLinearGradient(ctx,gg,CGPointZero,CGPointMake(0,sz.height),0);CGContextRestoreGState(ctx);CGGradientRelease(gg);
    CGFloat gc2[]={0.6,0.6,0.6,1, 0.4,0.4,0.4,1};
    CGGradientRef g2=CGGradientCreateWithColorComponents(cs,gc2,loc,2);
    CGContextSaveGState(ctx);CGContextAddRect(ctx,CGRectMake(sz.width/2,0,sz.width/2,sz.height));CGContextClip(ctx);
    CGContextDrawLinearGradient(ctx,g2,CGPointMake(sz.width/2,0),CGPointMake(sz.width/2,sz.height),0);CGContextRestoreGState(ctx);CGGradientRelease(g2);
    /* Border */
    CGFloat rad=sz.height/2;CGContextSetStrokeColorWithColor(ctx,[UIColor colorWithWhite:0.25 alpha:0.8].CGColor);CGContextSetLineWidth(ctx,1);
    CGContextBeginPath(ctx);CGContextMoveToPoint(ctx,rad,0);CGContextAddLineToPoint(ctx,sz.width-rad,0);
    CGContextAddArcToPoint(ctx,sz.width,0,sz.width,rad,rad);CGContextAddArcToPoint(ctx,sz.width,sz.height,sz.width-rad,sz.height,rad);
    CGContextAddLineToPoint(ctx,rad,sz.height);CGContextAddArcToPoint(ctx,0,sz.height,0,sz.height-rad,rad);
    CGContextAddArcToPoint(ctx,0,0,rad,0,rad);CGContextClosePath(ctx);CGContextStrokePath(ctx);
    UIImage *bgImg=UIGraphicsGetImageFromCurrentImageContext();UIGraphicsEndImageContext();
    UIImageView *bgIv=[[UIImageView alloc] initWithFrame:CGRectMake(0,0,sz.width,sz.height)];
    bgIv.image=bgIv;bgIv.layer.cornerRadius=rad;bgIv.layer.masksToBounds=YES;[self addSubview:bgIv];
    /* ON/OFF labels */
    _onLbl=[[UILabel alloc] initWithFrame:CGRectMake(5,2,30,sz.height-4)];
    _onLbl.text=@"ON";_onLbl.font=[UIFont boldSystemFontOfSize:13];_onLbl.textColor=[UIColor whiteColor];
    _onLbl.shadowColor=[UIColor colorWithWhite:0 alpha:0.4];_onLbl.shadowOffset=CGSizeMake(0,-1);
    _onLbl.textAlignment=NSTextAlignmentCenter;_onLbl.backgroundColor=[UIColor clearColor];[self addSubview:_onLbl];
    _offLbl=[[UILabel alloc] initWithFrame:CGRectMake(sz.width-35,2,30,sz.height-4)];
    _offLbl.text=@"OFF";_offLbl.font=[UIFont boldSystemFontOfSize:13];_offLbl.textColor=[UIColor whiteColor];
    _offLbl.shadowColor=[UIColor colorWithWhite:0 alpha:0.4];_offLbl.shadowOffset=CGSizeMake(0,-1);
    _offLbl.textAlignment=NSTextAlignmentCenter;_offLbl.backgroundColor=[UIColor clearColor];[self addSubview:_offLbl];
    /* Knob */
    CGFloat ks=sz.height-6;
    UIGraphicsBeginImageContextWithOptions(CGSizeMake(ks,ks),NO,1);
    CGContextRef kctx=UIGraphicsGetCurrentContext();CGFloat kr=ks/2;
    CGColorSpaceRef kcs=CGColorSpaceCreateDeviceRGB();
    CGFloat kl[]={0,1},kc[]={1,1,1,1, 0.85,0.85,0.85,1};
    CGGradientRef kg=CGGradientCreateWithColorComponents(kcs,kc,kl,2);
    CGContextDrawRadialGradient(kctx,kg,CGPointMake(kr,kr),0,CGPointMake(kr,kr),kr,0);CGGradientRelease(kg);
    CGContextSetFillColorWithColor(kctx,[UIColor colorWithWhite:1 alpha:0.4].CGColor);
    CGContextBeginPath(kctx);CGContextAddArc(kctx,kr,kr*0.6,kr*0.7,M_PI,2*M_PI,0);CGContextClosePath(kctx);CGContextFillPath(kctx);
    CGContextSetStrokeColorWithColor(kctx,[UIColor colorWithWhite:0.5 alpha:0.8].CGColor);CGContextSetLineWidth(kctx,1);
    CGContextAddEllipseInRect(kctx,CGRectMake(0.5,0.5,ks-1,ks-1));CGContextStrokePath(kctx);
    CGColorSpaceRelease(kcs);
    UIImage *kImg=UIGraphicsGetImageFromCurrentImageContext();UIGraphicsEndImageContext();
    _knob=[[UIImageView alloc] initWithFrame:CGRectMake(3,3,ks,ks)];
    _knob.image=kImg;_knob.layer.shadowColor=[UIColor blackColor].CGColor;_knob.layer.shadowOffset=CGSizeMake(0,2);
    _knob.layer.shadowOpacity=0.5;[self addSubview:_knob];
    [self updatePosition];
}
- (void)setIsOn:(BOOL)on {_isOn=on;[self updatePosition];}
- (void)updatePosition {
    CGSize sz=self.bounds.size;CGFloat ks=sz.height-6;
    [UIView animateWithDuration:0.2 animations:^{
        if(_isOn){_knob.frame=CGRectMake(sz.width-ks-3,3,ks,ks);_onLbl.alpha=1;_offLbl.alpha=0.4;}
        else{_knob.frame=CGRectMake(3,3,ks,ks);_onLbl.alpha=0.4;_offLbl.alpha=1;}
    }];
}
- (void)touchesEnded:(NSSet *)touches withEvent:(UIEvent *)e {_isOn=!_isOn;[self updatePosition];[self sendActionsForControlEvents:UIControlEventValueChanged];[super touchesEnded:touches withEvent:e];}
@end
