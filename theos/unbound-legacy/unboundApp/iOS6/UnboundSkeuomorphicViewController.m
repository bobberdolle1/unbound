#import "UnboundSkeuomorphicViewController.h"
#import "UnboundProxyManager.h"
#import "UnboundGlossyButton.h"
#import "UnboundLinenBackgroundView.h"
#import "UnboundLeatherPanelView.h"
#import "UnboundSkeuomorphicSwitch.h"

@interface UnboundSkeuomorphicViewController () {
    UnboundLinenBackgroundView *_bgView;
    UILabel *_titleLabel, *_subtitleLabel, *_statusLabel, *_statusDetailLabel;
    UIActivityIndicatorView *_spinner;
    UnboundLeatherPanelView *_statusPanel, *_controlPanel, *_settingsPanel;
    UnboundSkeuomorphicSwitch *_engineSwitch;
    UILabel *_engineLabel;
    UITextField *_portField;
    UISegmentedControl *_strategySeg;
    UnboundGlossyButton *_applyBtn, *_resetBtn, *_testBtn;
    BOOL _running;
}
@end

@implementation UnboundSkeuomorphicViewController

- (void)viewDidLoad {
    [super viewDidLoad];
    _running = NO;
    [self setup];
    [self loadStatus];
}

- (void)setup {
    /* Linen background */
    _bgView = [[UnboundLinenBackgroundView alloc] initWithFrame:self.view.bounds];
    _bgView.autoresizingMask = UIViewAutoresizingFlexibleWidth | UIViewAutoresizingFlexibleHeight;
    [self.view addSubview:_bgView];
    self.view.backgroundColor = [UIColor clearColor];

    /* Title */
    _titleLabel = [[UILabel alloc] initWithFrame:CGRectMake(0,80,self.view.bounds.size.width,36)];
    _titleLabel.text = @"UNBOUND";
    _titleLabel.font = [UIFont boldSystemFontOfSize:30];
    _titleLabel.textColor = [UIColor whiteColor];
    _titleLabel.shadowColor = [UIColor colorWithWhite:0 alpha:.7];
    _titleLabel.shadowOffset = CGSizeMake(0,-1);
    _titleLabel.textAlignment = NSTextAlignmentCenter;
    _titleLabel.backgroundColor = [UIColor clearColor];
    [self.view addSubview:_titleLabel];

    _subtitleLabel = [[UILabel alloc] initWithFrame:CGRectMake(0,118,self.view.bounds.size.width,20)];
    _subtitleLabel.text = @"Legacy DPI Bypass";
    _subtitleLabel.font = [UIFont systemFontOfSize:14];
    _subtitleLabel.textColor = [UIColor colorWithWhite:.85 alpha:1];
    _subtitleLabel.shadowColor = [UIColor colorWithWhite:0 alpha:.5];
    _subtitleLabel.shadowOffset = CGSizeMake(0,-1);
    _subtitleLabel.textAlignment = NSTextAlignmentCenter;
    _subtitleLabel.backgroundColor = [UIColor clearColor];
    [self.view addSubview:_subtitleLabel];

    /* Status panel (leather) */
    CGFloat pw = self.view.bounds.size.width - 40;
    _statusPanel = [[UnboundLeatherPanelView alloc] initWithFrame:CGRectMake(20,150,pw,80)];
    [self.view addSubview:_statusPanel];

    _statusLabel = [[UILabel alloc] initWithFrame:CGRectMake(20,15,200,24)];
    _statusLabel.font = [UIFont boldSystemFontOfSize:18];
    _statusLabel.backgroundColor = [UIColor clearColor];
    [_statusPanel addSubview:_statusLabel];

    _statusDetailLabel = [[UILabel alloc] initWithFrame:CGRectMake(20,42,pw-40,30)];
    _statusDetailLabel.font = [UIFont systemFontOfSize:13];
    _statusDetailLabel.textColor = [UIColor colorWithWhite:.7 alpha:1];
    _statusDetailLabel.numberOfLines = 2;
    _statusDetailLabel.backgroundColor = [UIColor clearColor];
    [_statusPanel addSubview:_statusDetailLabel];

    _spinner = [[UIActivityIndicatorView alloc] initWithActivityIndicatorStyle:UIActivityIndicatorViewStyleWhiteLarge];
    _spinner.center = CGPointMake(pw-35,40);
    _spinner.hidesWhenStopped = YES;
    [_statusPanel addSubview:_spinner];

    /* Control panel */
    _controlPanel = [[UnboundLeatherPanelView alloc] initWithFrame:CGRectMake(20,245,pw,60)];
    [self.view addSubview:_controlPanel];

    _engineLabel = [[UILabel alloc] initWithFrame:CGRectMake(20,18,180,24)];
    _engineLabel.text = @"Engine";
    _engineLabel.font = [UIFont boldSystemFontOfSize:17];
    _engineLabel.textColor = [UIColor whiteColor];
    _engineLabel.shadowColor = [UIColor blackColor].CGColor;
    _engineLabel.shadowOffset = CGSizeMake(0,-1);
    _engineLabel.backgroundColor = [UIColor clearColor];
    [_controlPanel addSubview:_engineLabel];

    _engineSwitch = [[UnboundSkeuomorphicSwitch alloc] initWithFrame:CGRectMake(pw-80,12,64,36)];
    [_engineSwitch addTarget:self action:@selector(toggleEngine) forControlEvents:UIControlEventValueChanged];
    [_controlPanel addSubview:_engineSwitch];

    /* Settings panel */
    _settingsPanel = [[UnboundLeatherPanelView alloc] initWithFrame:CGRectMake(20,320,pw,200)];
    [self.view addSubview:_settingsPanel];

    UILabel *portLbl = [[UILabel alloc] initWithFrame:CGRectMake(20,15,100,22)];
    portLbl.text = @"Listen Port";
    portLbl.font = [UIFont boldSystemFontOfSize:14];
    portLbl.textColor = [UIColor whiteColor];
    portLbl.shadowColor = [UIColor blackColor].CGColor;
    portLbl.shadowOffset = CGSizeMake(0,-1);
    portLbl.backgroundColor = [UIColor clearColor];
    [_settingsPanel addSubview:portLbl];

    _portField = [[UITextField alloc] initWithFrame:CGRectMake(130,10,80,30)];
    _portField.text = @"1993";
    _portField.keyboardType = UIKeyboardTypeNumberPad;
    _portField.textAlignment = NSTextAlignmentCenter;
    _portField.borderStyle = UITextBorderStyleBezel;
    _portField.backgroundColor = [UIColor whiteColor];
    _portField.layer.cornerRadius = 5;
    [_settingsPanel addSubview:_portField];

    UILabel *stratLbl = [[UILabel alloc] initWithFrame:CGRectMake(20,55,120,22)];
    stratLbl.text = @"Strategy";
    stratLbl.font = [UIFont boldSystemFontOfSize:14];
    stratLbl.textColor = [UIColor whiteColor];
    stratLbl.shadowColor = [UIColor blackColor].CGColor;
    stratLbl.shadowOffset = CGSizeMake(0,-1);
    stratLbl.backgroundColor = [UIColor clearColor];
    [_settingsPanel addSubview:stratLbl];

    _strategySeg = [[UISegmentedControl alloc] initWithItems:@[@"HTTP",@"HTTPS",@"Mixed",@"WS"]];
    _strategySeg.frame = CGRectMake(15,80,pw-30,35);
    _strategySeg.selectedSegmentIndex = 2;
    _strategySeg.tintColor = [UIColor colorWithRed:.25 green:.55 blue:.85 alpha:1];
    [_settingsPanel addSubview:_strategySeg];

    _applyBtn = [[UnboundGlossyButton alloc] initWithFrame:CGRectMake(15,130,(pw-45)/2,44)];
    _applyBtn.title = @"Apply";
    _applyBtn.buttonColor = [UIColor colorWithRed:.15 green:.65 blue:.15 alpha:1];
    [_applyBtn addTarget:self action:@selector(applyTapped) forControlEvents:UIControlEventTouchUpInside];
    [_settingsPanel addSubview:_applyBtn];

    _resetBtn = [[UnboundGlossyButton alloc] initWithFrame:CGRectMake(25+(pw-45)/2,130,(pw-45)/2,44)];
    _resetBtn.title = @"Reset";
    _resetBtn.buttonColor = [UIColor colorWithRed:.75 green:.2 blue:.2 alpha:1];
    [_resetBtn addTarget:self action:@selector(resetTapped) forControlEvents:UIControlEventTouchUpInside];
    [_settingsPanel addSubview:_resetBtn];

    _testBtn = [[UnboundGlossyButton alloc] initWithFrame:CGRectMake(15,182,pw-30,44)];
    _testBtn.title = @"Test Connection";
    _testBtn.buttonColor = [UIColor colorWithRed:.2 green:.5 blue:.8 alpha:1];
    [_testBtn addTarget:self action:@selector(testTapped) forControlEvents:UIControlEventTouchUpInside];
    [_settingsPanel addSubview:_testBtn];

    [self updateStatus];
}

- (void)toggleEngine {
    _running = _engineSwitch.isOn;
    if(_running){
        [_spinner startAnimating];
        [UnboundProxyManager.sharedManager startEngineWithPort:[_portField.text integerValue]
                                                    strategy:_strategySeg.selectedSegmentIndex
                                                  completion:^(BOOL ok, NSString *msg) {
            dispatch_async(dispatch_get_main_queue(), ^{
                [_spinner stopAnimating];
                if(!ok){_running=NO;_engineSwitch.isOn=NO;}
                [self updateStatus];
                if(!ok)[self alert:@"Error" msg:msg];
            });
        }];
    } else {
        [UnboundProxyManager.sharedManager stopEngineWithCompletion:^(BOOL ok) {
            dispatch_async(dispatch_get_main_queue(), ^{
                [self updateStatus];
                if(!ok)[self alert:@"Warning" msg:@"Engine may not have stopped cleanly."];
            });
        }];
    }
}

- (void)applyTapped {
    if(!_running){_engineSwitch.isOn=YES;[self toggleEngine];}
    else{_running=NO;[UnboundProxyManager.sharedManager stopEngineWithCompletion:^(BOOL o){
        dispatch_async(dispatch_get_main_queue(),^{_running=YES;[self toggleEngine];});
    }];}
}

- (void)resetTapped {
    _portField.text=@"1993";_strategySeg.selectedSegmentIndex=2;
    if(_running){_engineSwitch.isOn=NO;[self toggleEngine];}
}

- (void)testTapped {
    [_spinner startAnimating];
    [UnboundProxyManager.sharedManager testConnectionWithCompletion:^(BOOL ok,NSString *msg){
        dispatch_async(dispatch_get_main_queue(),^{[_spinner stopAnimating];[self alert:(ok?@"Success":@"Failed") msg:msg];});
    }];
}

- (void)loadStatus {
    [UnboundProxyManager.sharedManager getEngineStatusWithCompletion:^(BOOL running,NSInteger port,NSString *msg){
        dispatch_async(dispatch_get_main_queue(),^{
            _running=running;_engineSwitch.isOn=running;
            if(port>0)_portField.text=[NSString stringWithFormat:@"%ld",(long)port];
            [self updateStatus];
        });
    }];
}

- (void)updateStatus {
    if(_running){
        _statusLabel.text=@"● ACTIVE";_statusLabel.textColor=[UIColor colorWithRed:.3 green:.9 blue:.3 alpha:1];
        _statusDetailLabel.text=[NSString stringWithFormat:@"Proxy 127.0.0.1:%@",_portField.text];
    } else {
        _statusLabel.text=@"● INACTIVE";_statusLabel.textColor=[UIColor colorWithRed:.9 green:.3 blue:.3 alpha:1];
        _statusDetailLabel.text=@"Tap the switch to activate DPI bypass";
    }
}

- (void)alert:(NSString *)title msg:(NSString *)msg {
    UIAlertView *a=[[UIAlertView alloc] initWithTitle:title message:msg delegate:nil cancelButtonTitle:@"OK" otherButtonTitles:nil];
    [a show];
}

- (void)touchesBegan:(NSSet *)touches withEvent:(UIEvent *)event {[self.view endEditing:YES];}

@end
