package oniontransport

import (
	"bufio"
	"context"
	"crypto/rsa"
	"encoding/base32"
	"encoding/pem"
	"errors"
	"fmt"
	"net/textproto"
	"path"
	"runtime"
	"time"

	"github.com/yawning/bulb/utils/pkcs1"

	"golang.org/x/net/proxy"
	//"github.com/yawning/bulb"
	//"github.com/yawning/bulb/utils/pkcs1"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	tpt "github.com/libp2p/go-libp2p-transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
	mafmt "github.com/multiformats/go-multiaddr-fmt"
	manet "github.com/multiformats/go-multiaddr-net"
	"paidpiper.com/go-libp2p-onion-transport/tor"
)

// IsValidOnionMultiAddr is used to validate that a multiaddr
// is representing a Tor onion service
func IsValidOnionMultiAddr(a ma.Multiaddr) bool {
	if len(a.Protocols()) != 1 {
		return false
	}

	// check for correct network type
	if (a.Protocols()[0].Name != "onion3") && (a.Protocols()[0].Name != "onion") {
		return false
	}

	// split into onion address and port
	addr, err := a.ValueForProtocol(ma.P_ONION3)
	if err != nil {
		return false
	}
	split := strings.Split(addr, ":")
	if len(split) != 2 {
		return false
	}

	// onion3 address without the ".onion" substring
	if len(split[0]) != 56 {
		fmt.Println(split[0])
		return false
	}
	_, err = base32.StdEncoding.DecodeString(strings.ToUpper(split[0]))
	if err != nil {
		return false
	}

	// onion port number
	i, err := strconv.Atoi(split[1])
	if err != nil {
		return false
	}
	if i >= 65536 || i < 1 {
		return false
	}

	return true
}

// OnionTransport implements go-libp2p-transport's Transport interface
type OnionTransport struct {
	ctx           context.Context
	proxyAddress  string
	torConnection *tor.Tor
	startConf     *tor.StartConf
	//	controlConn *bulb.Conn
	auth         *proxy.Auth
	keysDir      string
	keys         map[string]*rsa.PrivateKey
	onlyOnion    bool
	laddr        ma.Multiaddr
	nonAnonymous bool

	// Connection upgrader for upgrading insecure stream connections to
	// secure multiplex connections.
	Upgrader *tptu.Upgrader
}

var transportInstance *OnionTransport = nil

// NewOnionTransport creates a OnionTransport
//
// controlNet and controlAddr contain the connecting information
// for the tor control port; either TCP or UNIX domain socket.
//
// controlPass contains the optional tor control password
//
// auth contains the socks proxy username and password
// keysDir is the key material for the Tor onion service.
//
// if onlyOnion is true the dialer will only be used to dial out on onion addresses
func NewOnionTransport(torExecutablePath string,
	torDataDir string,
	torConfigPath string,
	controlPass string,
	auth *proxy.Auth,
	keysDir string,
	upgrader *tptu.Upgrader,
	onlyOnion bool,
	supportNonAnonymousMode bool) (*OnionTransport, error) {

	if transportInstance != nil {
		return transportInstance, nil
	}

	logwriter := bufio.NewWriter(os.Stdout)
	if torDataDir == "" {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		torDataDir = path.Join(dirname, ".tor")
	}
	sp, cp, err := tor.ReadTorConfigsLines(torConfigPath)
	if err != nil {
		return nil, err
	}
	conf := &tor.StartConf{

		CreateProcess:     false,
		ExePath:           torExecutablePath,
		DataDir:           torDataDir,
		TorrcFile:         torConfigPath,
		ControlPort:       cp,
		NoAutoSocksPort:   true,
		DisableCookieAuth: false,
		DebugWriter:       logwriter,
		NoHush:            true,
	}

	o := &OnionTransport{
		ctx:          context.Background(),
		proxyAddress: fmt.Sprintf("127.0.0.1:%v", sp),
		startConf:    conf,
		auth:         auth,
		keysDir:      keysDir,
		onlyOnion:    onlyOnion,
		Upgrader:     upgrader,
		nonAnonymous: supportNonAnonymousMode,
	}
	keys, err := o.loadKeys()
	if err != nil {
		return nil, err
	}
	o.keys = keys
	err = o.createTorConnection()

	if err != nil {
		return nil, err
	}
	// Save the instance for future use
	transportInstance = o

	return o, nil
}

// OnionTransportC is a type alias for OnionTransport constructors, for use
// with libp2p.New
type OnionTransportC func(*tptu.Upgrader) (tpt.Transport, error)

// NewOnionTransportC is a convenience function that returns a function
// suitable for passing into libp2p.Transport for host configuration
func NewOnionTransportC(torExecutablePath string, torDataDir string, torConfigPath string, controlPass string, auth *proxy.Auth, keysDir string, onlyOnion bool, supportNonAnonymourMode bool) OnionTransportC {
	return func(upgrader *tptu.Upgrader) (tpt.Transport, error) {
		return NewOnionTransport(torExecutablePath, torDataDir, torConfigPath, controlPass, auth, keysDir, upgrader, onlyOnion, supportNonAnonymourMode)
	}
}

func (t *OnionTransport) createTorConnection() error {

	torConnection, err := tor.Start(t.ctx, t.startConf)

	if err != nil {
		return fmt.Errorf("unable to start Tor: %v", err)
	}
	torConnection.StopProcessOnClose = false
	runtime.SetFinalizer(torConnection, func(t *tor.Tor) {
		t.Close()
	})

	/*conn, err := bulb.Dial(controlNet, controlAddr)
	if err != nil {
		return nil, err
	}
	if err := conn.Authenticate(controlPass); err != nil {
		return nil, fmt.Errorf("authentication failed: %v", err)
	}
	*/
	t.torConnection = torConnection
	return nil
}

// Returns a proxy dialer gathered from the control interface.
// This isn't needed for the IPFS transport but it provides
// easy access to Tor for other functions.
func (t *OnionTransport) TorDialer(ctx context.Context) (proxy.Dialer, error) {

	conf := tor.DialConf{
		ProxyAddress:      t.proxyAddress,
		SkipEnableNetwork: false,
	}

	dialer, err := t.torConnection.Dialer(ctx, &conf)
	if err != nil {
		return nil, err
	}
	return dialer, nil
}

func (t *OnionTransport) Close() error {

	if t != nil {
		err := t.torConnection.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// loadKeys loads keys into our keys map from files in the keys directory
func (t *OnionTransport) onionKeyPath(absPath string) (string, error) {
	onionKeyPath := ""
	walkpath := func(path string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(path, ".onion_key") {
			onionKeyPath = path
		}
		return nil
	}

	err := filepath.Walk(absPath, walkpath)
	if err != nil {
		t.Debugf("walk error")
	}
	if onionKeyPath == "" {
		t.Debugf("onion_key file in %v not found", absPath)
	}
	return onionKeyPath, nil
}
func (t *OnionTransport) Debugf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (t *OnionTransport) loadKeys() (map[string]*rsa.PrivateKey, error) {
	keys := make(map[string]*rsa.PrivateKey)
	absPath, err := filepath.EvalSymlinks(t.keysDir)
	if err != nil {
		return nil, err
	}
	keyPath, err := t.onionKeyPath(absPath)
	if err != nil {
		return nil, fmt.Errorf("onion key path not found error: %v", err)
	}
	if keyPath != "" {
		key, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		file, err := os.Open(keyPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		onionName := strings.Replace(filepath.Base(file.Name()), ".onion_key", "", 1)
		block, _ := pem.Decode(key)
		privKey, _, err := pkcs1.DecodePrivateKeyDER(block.Bytes)
		if err != nil {
			return nil, err
		}
		keys[onionName] = privKey
	}

	return keys, err
}

// Dial dials a remote peer. It should try to reuse local listener
// addresses if possible but it may choose not to.
func (t *OnionTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (tpt.Conn, error) {
	fmt.Println("Call Dial", raddr.String())
	conf := tor.DialConf{
		ProxyAddress: t.proxyAddress,
	}

	dialer, err := t.torConnection.Dialer(ctx, &conf)

	if err != nil {
		var torError *textproto.Error

		if errors.As(err, &torError) {
			if torError.Code == 514 {
				fmt.Printf("Encountered 514 during tor transport dial (%s), recreating connection...\n", p)
				t.Close()
				errCreate := t.createTorConnection()
				if errCreate != nil {
					return nil, fmt.Errorf("Error recreating tor connection: %s", errCreate.Error())
				}

				return nil, err
			}
		}

		return nil, err
	}

	//netaddr, err := manet.ToNetAddr(raddr)

	var onionAddress string

	onionAddress, err = raddr.ValueForProtocol(ma.P_ONION3)

	if err != nil {
		onionAddress, err = raddr.ValueForProtocol(ma.P_ONION)

		if err != nil {
			return nil, err
		}
	}

	/*
		if err != nil {
			onionAddress, err = raddr.ValueForProtocol(ma.P_ONION3)
			if err != nil {
				return nil, err
			}
		}
	*/

	onionConn := OnionConn{
		transport: tpt.Transport(t),
		laddr:     t.laddr,
		raddr:     raddr,
	}
	if onionAddress != "" {
		split := strings.Split(onionAddress, ":")
		onionConn.Conn, err = dialer.Dial("tcp4", split[0]+".onion:"+split[1])
	} else {
		return nil, fmt.Errorf("onion address required, check comment")
		//onionConn.Conn, err = dialer.Dial(netaddr.Network(), netaddr.String())
	}
	if err != nil {
		return nil, err
	}
	return t.Upgrader.UpgradeOutbound(ctx, t, &onionConn, p)
}

// Listen listens on the passed multiaddr.
func (t *OnionTransport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {

	var netaddr string
	var err error

	// convert to net.Addr
	netaddr, err = laddr.ValueForProtocol(ma.P_ONION3)

	if err != nil {
		netaddr, err = laddr.ValueForProtocol(ma.P_ONION)

		if err != nil {
			return nil, err
		}
	}

	// retreive onion service virtport
	addr := strings.Split(netaddr, ":")
	if len(addr) != 2 {
		return nil, fmt.Errorf("failed to parse onion address")
	}

	// convert port string to int
	port, err := strconv.Atoi(addr[1])
	if err != nil {
		return nil, fmt.Errorf("failed to convert onion service port to int")
	}

	var onionKey *rsa.PrivateKey = nil

	/*
		onionKey, ok := t.keys[addr[0]]
		if !ok {
			return nil, fmt.Errorf("missing onion service key material for %s", addr[0])
		}
	*/

	listener := OnionListener{
		port:      uint16(port),
		key:       onionKey,
		laddr:     laddr,
		Upgrader:  t.Upgrader,
		transport: t,
	}

	listenCtx, _ := context.WithTimeout(context.Background(), 3*time.Minute)

	if t.torConnection == nil {
		return nil, fmt.Errorf("cannot listen on connection, torConnection is nil. (%d)", port)
	}

	onion, err := t.torConnection.Listen(listenCtx, &tor.ListenConf{Version3: true, NonAnonymous: t.nonAnonymous, RemotePorts: []int{port}})

	if err != nil {
		return nil, fmt.Errorf("error listening on torConnection port %d: %v", port, err)
	}

	var _ = onion.ID

	listener.listener = onion.LocalListener

	/*// setup bulb listener
	  _, err = pkcs1.OnionAddr(&onionKey.PublicKey)
	  if err != nil {
	  	return nil, fmt.Errorf("failed to derive onion ID: %v", err)
	  }
	  listener.listener, err = t.controlConn.Listener(uint16(port), onionKey)
	  if err != nil {
	  	return nil, err
	  }

	*/
	t.laddr = laddr

	return &listener, nil
}

// CanDial returns true if this transport knows how to dial the given
// multiaddr.
//
// Returning true does not guarantee that dialing this multiaddr will
// succeed. This function should *only* be used to preemptively filter
// out addresses that we can't dial.
func (t *OnionTransport) CanDial(a ma.Multiaddr) bool {
	if t.onlyOnion {
		// only dial out on onion addresses
		return IsValidOnionMultiAddr(a)
	} else {
		return IsValidOnionMultiAddr(a) || mafmt.TCP.Matches(a)
	}
}

// Protocols returns the list of terminal protocols this transport can dial.
func (t *OnionTransport) Protocols() []int {
	if !t.onlyOnion {
		return []int{ma.P_ONION, ma.P_ONION3, ma.P_TCP}
	} else {
		return []int{ma.P_ONION, ma.P_ONION3}
	}
}

// Proxy always returns false for the onion transport.
func (t *OnionTransport) Proxy() bool {
	return false
}

// OnionListener implements go-libp2p-transport's Listener interface
type OnionListener struct {
	port      uint16
	key       *rsa.PrivateKey
	laddr     ma.Multiaddr
	listener  net.Listener
	transport tpt.Transport
	Upgrader  *tptu.Upgrader
}

// Accept blocks until a connection is received returning
// go-libp2p-transport's Conn interface or an error if
// something went wrong
func (l *OnionListener) Accept() (tpt.Conn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	raddr, err := manet.FromNetAddr(conn.RemoteAddr())
	if err != nil {
		return nil, err
	}
	onionConn := OnionConn{
		Conn:      conn,
		transport: l.transport,
		laddr:     l.laddr,
		raddr:     raddr,
	}
	return l.Upgrader.UpgradeInbound(context.Background(), l.transport, &onionConn)
}

// Close shuts down the listener
func (l *OnionListener) Close() error {
	return l.listener.Close()
}

// Addr returns the net.Addr interface which represents
// the local multiaddr we are listening on
func (l *OnionListener) Addr() net.Addr {
	netaddr, _ := manet.ToNetAddr(l.laddr)
	return netaddr
}

// Multiaddr returns the local multiaddr we are listening on
func (l *OnionListener) Multiaddr() ma.Multiaddr {
	return l.laddr
}

// OnionConn implement's go-libp2p-transport's Conn interface
type OnionConn struct {
	net.Conn
	transport tpt.Transport
	laddr     ma.Multiaddr
	raddr     ma.Multiaddr
}

// Transport returns the OnionTransport associated
// with this OnionConn
func (c *OnionConn) Transport() tpt.Transport {
	return c.transport
}

// LocalMultiaddr returns the local multiaddr for this connection
func (c *OnionConn) LocalMultiaddr() ma.Multiaddr {
	return c.laddr
}

// RemoteMultiaddr returns the remote multiaddr for this connection
func (c *OnionConn) RemoteMultiaddr() ma.Multiaddr {
	return c.raddr
}
