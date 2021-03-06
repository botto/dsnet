package dsnet

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func Init() {
	_, err := os.Stat(CONFIG_FILE)

	if !os.IsNotExist(err) {
		ExitFail("Refusing to overwrite existing %s", CONFIG_FILE)
	}

	conf := DsnetConfig{
		PrivateKey:    GenerateJSONPrivateKey(),
		ListenPort:    DEFAULT_LISTEN_PORT,
		Network:       getPrivateNet(),
		Network6:      getULANet(),
		Peers:         []PeerConfig{},
		Domain:        "dsnet",
		ReportFile:    DEFAULT_REPORT_FILE,
		ExternalIP:    getExternalIP(),
		ExternalIP6:   getExternalIP6(),
		InterfaceName: DEFAULT_INTERFACE_NAME,
		Networks:      []JSONIPNet{},
	}

	conf.IP = conf.MustAllocateIP()
	conf.IP6 = conf.MustAllocateIP6()

	if len(conf.ExternalIP) == 0 && len(conf.ExternalIP6) == 0 {
		ExitFail("Could not determine any external IP, v4 or v6")
	}

	// DNS not set by default
	//conf.DNS = IP

	conf.MustSave()

	fmt.Printf("Config written to %s. Please check/edit.\n", CONFIG_FILE)
}

// get a random IPv4  /22 subnet on 10.0.0.0 (1023 hosts) (or /24?)
func getPrivateNet() JSONIPNet {
	rbs := make([]byte, 2)
	rand.Seed(time.Now().UTC().UnixNano())
	rand.Read(rbs)

	return JSONIPNet{
		IPNet: net.IPNet{
			net.IP{10, rbs[0], rbs[1] << 2, 0},
			net.IPMask{255, 255, 252, 0},
		},
	}
}

func getULANet() JSONIPNet {
	rbs := make([]byte, 5)
	rand.Seed(time.Now().UTC().UnixNano())
	rand.Read(rbs)

	// fd00 prefix with 40 bit global id and zero (16 bit) subnet ID
	return JSONIPNet{
		IPNet: net.IPNet{
			net.IP{0xfd, 0, rbs[0], rbs[1], rbs[2], rbs[3], rbs[4], 0, 0, 0, 0, 0, 0, 0, 0, 0},
			net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}
}

// TODO factor getExternalIP + getExternalIP6
func getExternalIP() net.IP {
	var IP net.IP
	// arbitrary external IP is used (one that's guaranteed to route outside.
	// In this case, Google's DNS server. Doesn't actually need to be online.)
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err == nil {
		defer conn.Close()

		localAddr := conn.LocalAddr().String()
		IP = net.ParseIP(strings.Split(localAddr, ":")[0])
		IP = IP.To4()

		if !(IP[0] == 10 || (IP[0] == 172 && IP[1] >= 16 && IP[1] <= 31) || (IP[0] == 192 && IP[1] == 168)) {
			// not private, so public
			return IP
		}
	}

	// detect private IP and use icanhazip.com instead
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://ipv4.icanhazip.com/")
	check(err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		check(err)
		IP = net.ParseIP(strings.TrimSpace(string(body)))
		return IP.To4()
	}

	return net.IP{}
}

func getExternalIP6() net.IP {
	var IP net.IP
	conn, err := net.Dial("udp", "2001:4860:4860::8888:53")
	if err == nil {
		defer conn.Close()

		localAddr := conn.LocalAddr().String()
		IP = net.ParseIP(strings.Split(localAddr, ":")[0])

		// check is not a ULA
		if IP[0] != 0xfd && IP[0] != 0xfc {
			return IP
		}
	}

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://ipv6.icanhazip.com/")
	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			check(err)
			IP = net.ParseIP(strings.TrimSpace(string(body)))
			return IP
		}
	}

	return net.IP{}
}
