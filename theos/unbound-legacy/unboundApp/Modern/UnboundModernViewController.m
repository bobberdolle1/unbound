#import "UnboundModernViewController.h"
#import "UnboundProxyManager.h"

@interface UnboundModernViewController () <UITableViewDelegate, UITableViewDataSource> { BOOL _running; NSInteger _strategy; NSString *_statusMsg; }
@property (nonatomic, strong) UITableView *tableView;
@end

@implementation UnboundModernViewController
- (void)viewDidLoad {
    [super viewDidLoad]; _running=NO; _strategy=2; _statusMsg=@"Engine inactive";
    self.title=@"Unbound"; self.view.backgroundColor=[UIColor systemGroupedBackgroundColor];
    [self setupNav]; [self setupTable]; [self loadStatus];
}
- (void)setupNav {
    if(@available(iOS 15,*)){
        UINavigationBarAppearance *a=[[UINavigationBarAppearance alloc]init];[a configureWithTransparentBackground];
        a.titleTextAttributes=@{NSFontAttributeName:[UIFont boldSystemFontOfSize:24],NSForegroundColorAttributeName:[UIColor labelColor]};
        a.largeTitleTextAttributes=@{NSFontAttributeName:[UIFont boldSystemFontOfSize:34],NSForegroundColorAttributeName:[UIColor labelColor]};
        self.navigationController.navigationBar.standardAppearance=a;self.navigationController.navigationBar.scrollEdgeAppearance=a;
    }
    self.navigationItem.largeTitleDisplayMode=UINavigationItemLargeTitleDisplayModeAlways;
    self.navigationItem.rightBarButtonItem=[[UIBarButtonItem alloc]initWithImage:[UIImage systemImageNamed:@"wifi.exclamationmark"] style:UIBarButtonItemStylePlain target:self action:@selector(showInfo)];
}
- (void)setupTable {
    self.tableView=[[UITableView alloc]initWithFrame:self.view.bounds style:UITableViewStyleGrouped];
    self.tableView.autoresizingMask=UIViewAutoresizingFlexibleWidth|UIViewAutoresizingFlexibleHeight;
    self.tableView.delegate=self;self.tableView.dataSource=self;
    self.tableView.tableFooterView=[[UIView alloc]init];
    UIRefreshControl *rc=[[UIRefreshControl alloc]init];[rc addTarget:self action:@selector(loadStatus) forControlEvents:UIControlEventValueChanged];
    self.tableView.refreshControl=rc;
    [self.view addSubview:self.tableView];
}
- (NSInteger)numberOfSectionsInTableView:(UITableView *)tv {return 4;}
- (NSInteger)tableView:(UITableView *)tv numberOfRowsInSection:(NSInteger)s {return s==0?1:s==1?1:s==2?3:2;}
- (NSString *)tableView:(UITableView *)tv titleForHeaderInSection:(NSInteger)s {return s==0?@"STATUS":s==1?@"ENGINE":s==2?@"CONFIGURATION":@"ACTIONS";}
- (UITableViewCell *)tableView:(UITableView *)tv cellForRowAtIndexPath:(NSIndexPath *)ip {
    UITableViewCell *c=[[UITableViewCell alloc]initWithStyle:UITableViewCellStyleSubtitle reuseIdentifier:nil];
    if(ip.section==0){
        c.textLabel.text=_statusMsg;c.detailTextLabel.text=_running?@"Running":@"Stopped";
        c.imageView.image=[UIImage systemImageNamed:_running?@"checkmark.circle.fill":@"xmark.circle.fill"];
        c.imageView.tintColor=_running?[UIColor systemGreenColor]:[UIColor systemRedColor];
    } else if(ip.section==1){
        c.textLabel.text=@"DPI Bypass Engine";c.detailTextLabel.text=_running?@"Active":@"Inactive";
        UISwitch *sw=[[UISwitch alloc]init];sw.on=_running;[sw addTarget:self action:@selector(toggleEngine:) forControlEvents:UIControlEventValueChanged];
        c.accessoryView=sw;
    } else if(ip.section==2){
        if(ip.row==0){c.textLabel.text=@"Listen Port";UITextField *f=[[UITextField alloc]initWithFrame:CGRectMake(0,0,80,30)];f.text=@"1993";f.keyboardType=UIKeyboardTypeNumberPad;f.textAlignment=NSTextAlignmentRight;c.accessoryView=f;}
        else if(ip.row==1){c.textLabel.text=@"Strategy";UISegmentedControl *s=[[UISegmentedControl alloc]initWithItems:@[@"HTTP",@"HTTPS",@"Mixed",@"WS"]];s.selectedSegmentIndex=(NSUInteger)_strategy;[s addTarget:self action:@selector(strategyChanged:) forControlEvents:UIControlEventValueChanged];c.accessoryView=s;}
        else{c.textLabel.text=@"Auto-start on Boot";UISwitch *s=[[UISwitch alloc]init];s.on=YES;c.accessoryView=s;}
    } else {
        c.textLabel.text=ip.row==0?@"Test Connection":@"View Logs";c.textLabel.textColor=[UIColor systemBlueColor];c.accessoryType=UITableViewCellAccessoryDisclosureIndicator;
    }
    return c;
}
- (void)toggleEngine:(UISwitch *)sw {_running=sw.on; _running?[self startEngine]:[self stopEngine];}
- (void)strategyChanged:(UISegmentedControl *)s {_strategy=s.selectedSegmentIndex;}
- (void)startEngine {
    [self.tableView setUserInteractionEnabled:NO];
    [UnboundProxyManager.sharedManager startEngineWithPort:1993 strategy:_strategy completion:^(BOOL ok,NSString *msg){
        dispatch_async(dispatch_get_main_queue(),^{[self.tableView setUserInteractionEnabled:YES];_running=ok;_statusMsg=ok?msg:@"Failed";[self.tableView reloadData];
            if(!ok)[self alert:@"Error" msg:msg];});}];
}
- (void)stopEngine {[UnboundProxyManager.sharedManager stopEngineWithCompletion:^(BOOL o){dispatch_async(dispatch_get_main_queue(),^{_running=NO;_statusMsg=@"Stopped";[self.tableView reloadData];});}];}
- (void)loadStatus {[UnboundProxyManager.sharedManager getEngineStatusWithCompletion:^(BOOL r,NSInteger p,NSString *m){dispatch_async(dispatch_get_main_queue(),^{_running=r;_statusMsg=m;[self.tableView reloadData];[self.tableView.refreshControl endRefreshing];});}];}
- (void)showInfo {[self alert:@"Unbound Legacy" msg:@"DPI/Censorship Bypass v1.0.0"];}
- (void)tableView:(UITableView *)tv didSelectRowAtIndexPath:(NSIndexPath *)ip {
    [tv deselectRowAtIndexPath:ip animated:YES];
    if(ip.section==3&&ip.row==0)[UnboundProxyManager.sharedManager testConnectionWithCompletion:^(BOOL o,NSString *m){dispatch_async(dispatch_get_main_queue(),^{[self alert:(o?@"OK":@"Failed") msg:m];});}];
}
- (void)alert:(NSString *)t msg:(NSString *)m {UIAlertController *a=[UIAlertController alertControllerWithTitle:t message:m preferredStyle:UIAlertControllerStyleAlert];[a addAction:[UIAlertAction actionWithTitle:@"OK" style:UIAlertActionStyleDefault handler:nil]];[self presentViewController:a animated:YES completion:nil];}
@end
