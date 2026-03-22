package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	XrayNodesFileName = "xray_nodes.json"
	XrayConfigName    = "xray_config.json"
	SubscriptionTimeout = 15 * time.Second
)

type XrayNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	UUID     string `json:"uuid"`
	Flow     string `json:"flow"`
	Security string `json:"security"`
	SNI      string `json:"sni"`
	FP       string `json:"fp"`
	PBK      string `json:"pbk"`
	SID      string `json:"sid"`
	Type     string `json:"type"`
	RawURI   string `json:"rawUri"`
}

type XrayConfig struct {
	Log struct {
		Loglevel string `json:"loglevel"`
	} `json:"log"`
	Inbounds []struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Settings struct {
			Auth           string `json:"auth,omitempty"`
			UDP            bool   `json:"udp,omitempty"`
			AllowTransparent bool `json:"allowTransparent,omitempty"`
		} `json:"settings"`
	} `json:"inbounds"`
	Outbounds []struct {
		Protocol string `json:"protocol"`
		Settings struct {
			Vnext []struct {
				Address string `json:"address"`
				Port    int    `json:"port"`
				Users   []struct {
					ID         string `json:"id"`
					Flow       string `json:"flow,omitempty"`
					Encryption string `json:"encryption"`
				} `json:"users"`
			} `json:"vnext,omitempty"`
		} `json:"settings"`
		StreamSettings struct {
			Network  string `json:"network"`
			Security string `json:"security"`
			RealitySettings struct {
				Show        bool   `json:"show"`
				Fingerprint string `json:"fingerprint"`
				ServerName  string `json:"serverName"`
				PublicKey   string `json:"publicKey"`
				ShortID     string `json:"shortId"`
				SpiderX     string `json:"spiderX"`
			} `json:"realitySettings,omitempty"`
		} `json:"streamSettings"`
	} `json:"outbounds"`
}

func AddSubscription(link string) ([]XrayNode, error) {
	var rawContent string
	
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		client := &http.Client{Timeout: SubscriptionTimeout}
		resp, err := http.NewRequest("GET", link, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		response, err := client.Do(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch subscription: %w", err)
		}
		defer response.Body.Close()
		
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("subscription server returned status %d", response.StatusCode)
		}
		
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		
		rawContent = string(bodyBytes)
		
		decoded, err := base64.StdEncoding.DecodeString(rawContent)
		if err == nil {
			rawContent = string(decoded)
		}
	} else if strings.HasPrefix(link, "vless://") {
		rawContent = link
	} else {
		return nil, fmt.Errorf("invalid subscription link or vless:// URI")
	}
	
	lines := strings.Split(rawContent, "\n")
	nodes := make([]XrayNode, 0)
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "vless://") {
			continue
		}
		
		node, err := parseVlessURI(line)
		if err != nil {
			continue
		}
		
		nodes = append(nodes, node)
	}
	
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid vless:// nodes found in subscription")
	}
	
	if err := saveXrayNodes(nodes); err != nil {
		return nil, fmt.Errorf("failed to save nodes: %w", err)
	}
	
	return nodes, nil
}

func parseVlessURI(uri string) (XrayNode, error) {
	uri = strings.TrimPrefix(uri, "vless://")
	
	parts := strings.SplitN(uri, "@", 2)
	if len(parts) != 2 {
		return XrayNode{}, fmt.Errorf("invalid vless URI format")
	}
	
	uuid := parts[0]
	
	remaining := parts[1]
	addressParts := strings.SplitN(remaining, "?", 2)
	if len(addressParts) < 1 {
		return XrayNode{}, fmt.Errorf("invalid vless URI format")
	}
	
	addressPort := addressParts[0]
	hostPort := strings.Split(addressPort, ":")
	if len(hostPort) != 2 {
		return XrayNode{}, fmt.Errorf("invalid address:port format")
	}
	
	address := hostPort[0]
	port := 0
	fmt.Sscanf(hostPort[1], "%d", &port)
	
	params := url.Values{}
	name := ""
	
	if len(addressParts) == 2 {
		queryAndName := addressParts[1]
		nameParts := strings.SplitN(queryAndName, "#", 2)
		
		if len(nameParts) == 2 {
			name, _ = url.QueryUnescape(nameParts[1])
		}
		
		params, _ = url.ParseQuery(nameParts[0])
	}
	
	if name == "" {
		name = fmt.Sprintf("%s:%d", address, port)
	}
	
	node := XrayNode{
		ID:       fmt.Sprintf("%s_%d", address, time.Now().Unix()),
		Name:     name,
		Address:  address,
		Port:     port,
		UUID:     uuid,
		Flow:     params.Get("flow"),
		Security: params.Get("security"),
		SNI:      params.Get("sni"),
		FP:       params.Get("fp"),
		PBK:      params.Get("pbk"),
		SID:      params.Get("sid"),
		Type:     params.Get("type"),
		RawURI:   uri,
	}
	
	return node, nil
}

func GetXrayNodes() ([]XrayNode, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}
	
	nodesPath := filepath.Join(configDir, XrayNodesFileName)
	
	data, err := os.ReadFile(nodesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []XrayNode{}, nil
		}
		return nil, err
	}
	
	var nodes []XrayNode
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, err
	}
	
	return nodes, nil
}

func saveXrayNodes(nodes []XrayNode) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	
	nodesPath := filepath.Join(configDir, XrayNodesFileName)
	
	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(nodesPath, data, 0644)
}

func GenerateXrayConfig(nodeID string) error {
	nodes, err := GetXrayNodes()
	if err != nil {
		return err
	}
	
	var selectedNode *XrayNode
	for i := range nodes {
		if nodes[i].ID == nodeID {
			selectedNode = &nodes[i]
			break
		}
	}
	
	if selectedNode == nil {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	
	config := XrayConfig{}
	config.Log.Loglevel = "warning"
	
	config.Inbounds = []struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Settings struct {
			Auth           string `json:"auth,omitempty"`
			UDP            bool   `json:"udp,omitempty"`
			AllowTransparent bool `json:"allowTransparent,omitempty"`
		} `json:"settings"`
	}{
		{
			Port:     10808,
			Protocol: "socks",
			Settings: struct {
				Auth           string `json:"auth,omitempty"`
				UDP            bool   `json:"udp,omitempty"`
				AllowTransparent bool `json:"allowTransparent,omitempty"`
			}{
				Auth: "noauth",
				UDP:  true,
			},
		},
	}
	
	config.Outbounds = []struct {
		Protocol string `json:"protocol"`
		Settings struct {
			Vnext []struct {
				Address string `json:"address"`
				Port    int    `json:"port"`
				Users   []struct {
					ID         string `json:"id"`
					Flow       string `json:"flow,omitempty"`
					Encryption string `json:"encryption"`
				} `json:"users"`
			} `json:"vnext,omitempty"`
		} `json:"settings"`
		StreamSettings struct {
			Network  string `json:"network"`
			Security string `json:"security"`
			RealitySettings struct {
				Show        bool   `json:"show"`
				Fingerprint string `json:"fingerprint"`
				ServerName  string `json:"serverName"`
				PublicKey   string `json:"publicKey"`
				ShortID     string `json:"shortId"`
				SpiderX     string `json:"spiderX"`
			} `json:"realitySettings,omitempty"`
		} `json:"streamSettings"`
	}{
		{
			Protocol: "vless",
			Settings: struct {
				Vnext []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
					Users   []struct {
						ID         string `json:"id"`
						Flow       string `json:"flow,omitempty"`
						Encryption string `json:"encryption"`
					} `json:"users"`
				} `json:"vnext,omitempty"`
			}{
				Vnext: []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
					Users   []struct {
						ID         string `json:"id"`
						Flow       string `json:"flow,omitempty"`
						Encryption string `json:"encryption"`
					} `json:"users"`
				}{
					{
						Address: selectedNode.Address,
						Port:    selectedNode.Port,
						Users: []struct {
							ID         string `json:"id"`
							Flow       string `json:"flow,omitempty"`
							Encryption string `json:"encryption"`
						}{
							{
								ID:         selectedNode.UUID,
								Flow:       selectedNode.Flow,
								Encryption: "none",
							},
						},
					},
				},
			},
			StreamSettings: struct {
				Network  string `json:"network"`
				Security string `json:"security"`
				RealitySettings struct {
					Show        bool   `json:"show"`
					Fingerprint string `json:"fingerprint"`
					ServerName  string `json:"serverName"`
					PublicKey   string `json:"publicKey"`
					ShortID     string `json:"shortId"`
					SpiderX     string `json:"spiderX"`
				} `json:"realitySettings,omitempty"`
			}{
				Network:  "tcp",
				Security: selectedNode.Security,
				RealitySettings: struct {
					Show        bool   `json:"show"`
					Fingerprint string `json:"fingerprint"`
					ServerName  string `json:"serverName"`
					PublicKey   string `json:"publicKey"`
					ShortID     string `json:"shortId"`
					SpiderX     string `json:"spiderX"`
				}{
					Show:        false,
					Fingerprint: selectedNode.FP,
					ServerName:  selectedNode.SNI,
					PublicKey:   selectedNode.PBK,
					ShortID:     selectedNode.SID,
					SpiderX:     "",
				},
			},
		},
	}
	
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	
	configPath := filepath.Join(configDir, XrayConfigName)
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}

func GetXrayConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, XrayConfigName), nil
}
