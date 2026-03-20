#!/system/bin/sh

MODDIR=${0%/*}
BINDIR="$MODDIR/bin"
ARCH=$(getprop ro.product.cpu.abi)

if [ "$ARCH" = "arm64-v8a" ]; then
    NFQWS="$BINDIR/nfqws_arm64"
elif [ "$ARCH" = "armeabi-v7a" ]; then
    NFQWS="$BINDIR/nfqws_arm"
else
    echo "Unsupported architecture: $ARCH" > /data/local/tmp/unbound_error.log
    exit 1
fi

chmod 755 "$NFQWS"

QUEUE_NUM=200
PROFILE="${UNBOUND_PROFILE:-ultimate}"

setup_iptables() {
    iptables -t mangle -F POSTROUTING 2>/dev/null
    ip6tables -t mangle -F POSTROUTING 2>/dev/null

    case "$PROFILE" in
        ultimate)
            iptables -t mangle -I POSTROUTING -p tcp -m multiport --dports 80,443,5222,5223,5228,4244 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            iptables -t mangle -I POSTROUTING -p udp -m multiport --dports 443,3478,50000:65535 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            ;;
        discord)
            iptables -t mangle -I POSTROUTING -p tcp --dport 443 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            iptables -t mangle -I POSTROUTING -p udp -m multiport --dports 443,3478,50000:65535 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            ;;
        youtube)
            iptables -t mangle -I POSTROUTING -p tcp -m multiport --dports 80,443 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            iptables -t mangle -I POSTROUTING -p udp --dport 443 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            ;;
        telegram)
            iptables -t mangle -I POSTROUTING -p tcp -m multiport --dports 443,5222,5223,5228 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            iptables -t mangle -I POSTROUTING -p udp --dport 443 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            ;;
        *)
            iptables -t mangle -I POSTROUTING -p tcp --dport 443 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            iptables -t mangle -I POSTROUTING -p udp --dport 443 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num $QUEUE_NUM --queue-bypass
            ;;
    esac
}

start_nfqws() {
    case "$PROFILE" in
        ultimate)
            "$NFQWS" --qnum=$QUEUE_NUM --daemon --user=shell \
                --filter-tcp=80 --dpi-desync=fake,multisplit --dpi-desync-split-pos=method+2 --dpi-desync-fooling=md5sig --new \
                --filter-tcp=443 --dpi-desync=fake,multidisorder --dpi-desync-split-pos=1,midsld --dpi-desync-fooling=badseq,md5sig --new \
                --filter-tcp=5222,5223,5228,4244 --dpi-desync=split --dpi-desync-split-pos=2 --dpi-desync=disorder --new \
                --filter-udp=443 --dpi-desync=fake --dpi-desync-repeats=10 --dpi-desync-udplen-increment=2 --new \
                --filter-udp=3478,50000-65535 --dpi-desync=fake --dpi-desync-repeats=8
            ;;
        discord)
            "$NFQWS" --qnum=$QUEUE_NUM --daemon --user=shell \
                --filter-tcp=443 --dpi-desync=fake,split --dpi-desync-split-pos=1 --dpi-desync-fooling=md5sig --new \
                --filter-udp=443 --dpi-desync=fake --dpi-desync-repeats=10 --new \
                --filter-udp=3478 --dpi-desync=fake --dpi-desync-repeats=8 --new \
                --filter-udp=50000-65535 --dpi-desync=fake --dpi-desync-repeats=8
            ;;
        youtube)
            "$NFQWS" --qnum=$QUEUE_NUM --daemon --user=shell \
                --filter-tcp=80 --dpi-desync=fake,multisplit --dpi-desync-split-pos=method+2 --dpi-desync-fooling=md5sig --new \
                --filter-tcp=443 --dpi-desync=fake,multisplit --dpi-desync-split-pos=1,midsld --dpi-desync-fooling=md5sig --new \
                --filter-udp=443 --dpi-desync=fake --dpi-desync-repeats=12 --dpi-desync-udplen-increment=2
            ;;
        telegram)
            "$NFQWS" --qnum=$QUEUE_NUM --daemon --user=shell \
                --filter-tcp=443 --dpi-desync=fake,split --dpi-desync-split-pos=1 --dpi-desync-fooling=md5sig --new \
                --filter-tcp=5222,5223,5228 --dpi-desync=split --dpi-desync-split-pos=2 --dpi-desync=disorder --new \
                --filter-udp=443 --dpi-desync=fake --dpi-desync-repeats=8
            ;;
        *)
            "$NFQWS" --qnum=$QUEUE_NUM --daemon --user=shell \
                --filter-tcp=443 --dpi-desync=fake,split --dpi-desync-split-pos=1 --dpi-desync-fooling=md5sig --new \
                --filter-udp=443 --dpi-desync=fake --dpi-desync-repeats=6
            ;;
    esac
}

killall -9 nfqws_arm64 nfqws_arm 2>/dev/null

setup_iptables
start_nfqws

echo "Unbound DPI Bypass started with profile: $PROFILE" > /data/local/tmp/unbound_status.log
