#import "UnboundGlossyButton.h"
@implementation UnboundGlossyButton { UIColor *_color; NSString *_title; BOOL _pressed; }
@synthesize buttonColor=_color, title=_title;
- (instancetype)initWithFrame:(CGRect)frame {
    self=[super initWithFrame:frame]; if(self){_color=[UIColor colorWithRed:.2 green:.5 blue:.85 alpha:1];_title=@"";_pressed=NO;
        self.backgroundColor=[UIColor clearColor];self.layer.cornerRadius=8;self.layer.masksToBounds=YES;
        self.layer.shadowColor=[UIColor blackColor].CGColor;self.layer.shadowOffset=CGSizeMake(0,2);self.layer.shadowOpacity=0.4;}
    return self;
}
- (void)touchesBegan:(NSSet *)touches withEvent:(UIEvent *)e {_pressed=YES;[self setNeedsDisplay];[super touchesBegan:touches withEvent:e];}
- (void)touchesEnded:(NSSet *)touches withEvent:(UIEvent *)e {_pressed=NO;[self setNeedsDisplay];[super touchesEnded:touches withEvent:e];}
- (void)touchesCancelled:(NSSet *)touches withEvent:(UIEvent *)e {_pressed=NO;[self setNeedsDisplay];[super touchesCancelled:touches withEvent:e];}
- (void)drawRect:(CGRect)rect {
    CGContextRef ctx=UIGraphicsGetCurrentContext(); CGSize sz=self.bounds.size; CGFloat r=8;
    const CGFloat *c=CGColorGetComponents(_color.CGColor);
    CGColorSpaceRef cs=CGColorSpaceCreateDeviceRGB();
    if(_pressed){
        CGFloat loc[]={0,1},comp[]={c[0]*0.7,c[1]*0.7,c[2]*0.7,1, c[0]*0.5,c[1]*0.5,c[2]*0.5,1};
        CGGradientRef g=CGGradientCreateWithColorComponents(cs,comp,loc,2);
        CGContextDrawLinearGradient(ctx,g,CGPointZero,CGPointMake(0,sz.height),0);CGGradientRelease(g);
    } else {
        CGFloat loc[]={0,0.5,1},comp[]={MIN(c[0]+0.2,1),MIN(c[1]+0.2,1),MIN(c[2]+0.2,1),1, c[0],c[1],c[2],1, c[0]*0.8,c[1]*0.8,c[2]*0.8,1};
        CGGradientRef g=CGGradientCreateWithColorComponents(cs,comp,loc,2);
        CGContextDrawLinearGradient(ctx,g,CGPointZero,CGPointMake(0,sz.height),0);CGGradientRelease(g);
        /* Gloss */
        CGContextSaveGState(ctx);CGContextBeginPath(ctx);
        CGContextMoveToPoint(ctx,r,0);CGContextAddLineToPoint(ctx,sz.width-r,0);
        CGContextAddArcToPoint(ctx,sz.width,0,sz.width,r,r);CGContextAddLineToPoint(ctx,sz.width,sz.height*0.5);
        CGContextAddLineToPoint(ctx,0,sz.height*0.5);CGContextAddLineToPoint(ctx,0,r);
        CGContextAddArcToPoint(ctx,0,0,r,0,r);CGContextClosePath(ctx);CGContextClip(ctx);
        CGFloat gl[]={0,1},gc[]={1,1,1,0.35, 1,1,1,0.02};
        CGGradientRef gg=CGGradientCreateWithColorComponents(cs,gc,gl,2);
        CGContextDrawLinearGradient(ctx,gg,CGPointMake(sz.width/2,0),CGPointMake(sz.width/2,sz.height*0.5),0);
        CGGradientRelease(gg);CGContextRestoreGState(ctx);
    }
    /* Border */
    CGContextSetStrokeColorWithColor(ctx,[UIColor colorWithWhite:0.1 alpha:0.6].CGColor);CGContextSetLineWidth(ctx,1);
    CGContextBeginPath(ctx);CGContextMoveToPoint(ctx,r,0);CGContextAddLineToPoint(ctx,sz.width-r,0);
    CGContextAddArcToPoint(ctx,sz.width,0,sz.width,r,r);CGContextAddLineToPoint(ctx,sz.width,sz.height-r);
    CGContextAddArcToPoint(ctx,sz.width,sz.height,sz.width-r,sz.height,r);CGContextAddLineToPoint(ctx,r,sz.height);
    CGContextAddArcToPoint(ctx,0,sz.height,0,sz.height-r,r);CGContextAddLineToPoint(ctx,0,r);
    CGContextAddArcToPoint(ctx,0,0,r,0,r);CGContextClosePath(ctx);CGContextStrokePath(ctx);
    CGColorSpaceRelease(cs);
    /* Title */
    if(_title.length>0){
        NSShadow *sh=[[NSShadow alloc]init];sh.shadowColor=[UIColor colorWithWhite:0 alpha:0.6];sh.shadowOffset=CGSizeMake(0,-1);
        NSDictionary *a=@{NSFontAttributeName:[UIFont boldSystemFontOfSize:17],NSForegroundColorAttributeName:_pressed?[UIColor colorWithWhite:0.7 alpha:1]:[UIColor whiteColor],NSShadowAttributeName:sh};
        CGSize ts=[_title sizeWithAttributes:a];
        [_title drawInRect:CGRectMake((sz.width-ts.width)/2,(sz.height-ts.height)/2,ts.width,ts.height) withAttributes:a];
    }
}
@end
