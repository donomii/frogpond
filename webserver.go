package frogpond

import (
    //"embed"

    "context"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/signal"
    "sort"
    "sync"
    "syscall"

    "github.com/google/uuid"

    //"net/url"
    "strings"
    "time"

    "github.com/donomii/goof"
)

var ThisNode *Node

var Info InfoStruct
var wantUpdatePeers bool

type Service struct {
	Name        string
	Ip          string
	Port        int
	Protocol    string
	Description string
	Global      bool
	Path        string
}

var Configuration Config

type InfoStruct struct {
	GUID     string
	Name     string
	Services []Service
}

type HostService struct {
	GUID     string
	Ip       string
	Ports    []uint
	Port     uint
	Services []Service
	Name     string
	LastSeen time.Time
	Path     string
}

type HostServiceList []*HostService

func (a HostServiceList) Len() int           { return len(a) }
func (a HostServiceList) Less(i, j int) bool { return a[i].Ip < a[j].Ip }
func (a HostServiceList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func Hosts2Json() []byte {
    out, _ := json.Marshal(AllHosts())
    // Return a copy to ensure the buffer isn't reused elsewhere
    copied := make([]byte, len(out))
    copy(copied, out)
    return copied
}

var HostsMap map[string]*HostService
var ThisHost *HostService

var HostListLock = sync.Mutex{}

func AllHosts() []*HostService {
	HostListLock.Lock()
	defer HostListLock.Unlock()
	var hosts []*HostService
	for _, h := range HostsMap {
		hosts = append(hosts, h)
	}
	sort.Sort(HostServiceList(hosts))
	return hosts
}

func AddHost(h *HostService) {
    HostListLock.Lock()
    defer HostListLock.Unlock()
    if h.Port == 0 {
        log.Printf("Bad struct: %+v", h)
        // Reject invalid host entries with zero port instead of panicking
        return
    }
	if h.GUID == "" {
		log.Printf("Host %v has no GUID!", h.Ip)

	}

	if HostsMap == nil {
		HostsMap = make(map[string]*HostService)
	}
	v := h
	if v != nil { //Who is putting nils on the host list?

		//If v.Lastseen is more than an hour ago, remove it
		//if time.Since(v.LastSeen) > time.Hour {
		//	return
		//	}

		if _, ok := HostsMap[v.GUID]; !ok { //If we never saw it before
			HostsMap[v.GUID] = v
		} else {
			if v.LastSeen.After(HostsMap[v.GUID].LastSeen) { //If it is more recent than our copy
				HostsMap[v.GUID] = v
			}
		}

	}

	HostsMap[h.GUID] = h
}

func AddHosts(hs []*HostService) {
	for _, h := range hs {
		AddHost(h)
	}
}

// Attempt to retrieve an ipv4 or ipv6 address from a string
// ExtractIpv6 attempts to retrieve an IPv4 or IPv6 address from a string
// that may include a port. For IPv6 addresses, it supports bracketed forms.
func ExtractIpv6(ip string) string {
    ip = strings.TrimSpace(ip)
    if strings.Contains(ip, "[") {
        //string contains ip and port
        if strings.Contains(ip, "]:") {
            bits := strings.Split(ip, "]:")
            ip = bits[0]
            ip = ip[1:]
            return ip
        }

        ip = ip[1:]
        ip = ip[:len(ip)-1]
        return ip
    }

	if strings.Contains(ip, ".") {
		bits := strings.Split(ip, ":")
		//drop last bit, which is hopefully the port
		addr := bits[:len(bits)-1]
		ip = strings.Join(addr, ":")
		return ip
	}

    return ip
}

// Handle an incoming call to exchange frogpond data
func swapdata(w http.ResponseWriter, req *http.Request) {

    // Get remote ip address from connection
    ip := ExtractIpv6(req.RemoteAddr)

	// Read a json struct from the request body
	var data []DataPoint
	req.ParseForm()

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
        return
    }
    err = json.Unmarshal(body, &data)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
        return
    }

	// Add the remote ip to the list of hosts
	log.Println("Received swapdata contact from:", ip)
	// Print data
	/*
		for _, dp := range data {
			log.Printf("%v last seen at %v\n", string(dp.Key), dp.Updated)
		}
	*/
	ThisNode.applyUpdate(data)

    w.Header().Set("Content-Type", "application/json")
    if ThisNode.EnablePullData {
        _, _ = w.Write(ThisNode.JsonDump())
    } else {
        _, _ = w.Write([]byte("[]"))
    }
}

// Handle an incoming call to announce a peer
func announce(w http.ResponseWriter, req *http.Request) {

    //Get remote ip address from connection
    ip := ExtractIpv6(req.RemoteAddr)
	//Read a json struct from the request body
	var data HostService
	req.ParseForm()

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
        return
    }
    err = json.Unmarshal(body, &data)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
        return
    }

	//Add the remote ip to the list of hosts
	log.Println("Received annoucement from:", ip)
	//log.Printf("Received announce data: %+v\n", data)

	data.Ip = ip
	data.LastSeen = time.Now()

	if data.Port == 0 {
		data.Port = 16002
	}
	AddHost(&data)
	prefix := fmt.Sprintf("/frogpond/hosts/%v/", data.Ip)
	ThisNode.SetDataPointWithPrefix_str(prefix, "guid", data.GUID)
	ThisNode.SetDataPointWithPrefix_str(prefix, "name", data.Name)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "port", data.Port)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "lastseen", data.LastSeen)
	ThisNode.SetDataPointWithPrefix_str(prefix, "ip", data.Ip)
	ThisNode.SetDataPointWithPrefix_str(prefix, "path", data.Path)

	prefix = fmt.Sprintf("/frogpond/hosts/%v/", data.GUID)
	ThisNode.SetDataPointWithPrefix_str(prefix, "guid", data.GUID)
	ThisNode.SetDataPointWithPrefix_str(prefix, "name", data.Name)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "port", data.Port)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "lastseen", data.LastSeen)
	ThisNode.SetDataPointWithPrefix_str(prefix, "ip", data.Ip)
	ThisNode.SetDataPointWithPrefix_str(prefix, "path", data.Path)

	UpdatePeersData()
	UpdatePeers()
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    _, _ = w.Write([]byte("OK"))
}

// Handle an incoming call to exchange hosts data
func contact(w http.ResponseWriter, req *http.Request) {

	//Get remote ip address from connection
    ip := ExtractIpv6(req.RemoteAddr)

	//Read a json struct from the request body
	var data []*HostService
	req.ParseForm()

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
        return
    }
    err = json.Unmarshal(body, &data)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
        return
    }

	//Add the remote ip to the list of hosts
	log.Println("Received contact from:", ip)
	/*log.Printf("Received hosts list: %+v\n", data)
	//Print data


	for _, host := range data {

		log.Printf("%v:%v last seen at %v\n", host.Ip, host.Port, host.LastSeen)
	}
	*/
	AddHosts(data)

    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write(Hosts2Json())
}

func debugPage(w http.ResponseWriter, req *http.Request) {
	str := fmt.Sprintf(`
		<h1>Debug</h1>
		Executable path: %v
		Current working directory: %v
		`, goof.ExecutablePath(), goof.Cwd())

    fmt.Fprint(w, str)
}

// Publish our public information (port, services, etc)
func public_info(w http.ResponseWriter, req *http.Request) {
    out, _ := json.Marshal(Info)
    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write(out)
}

// Send our host list to a peer
// httpClient is used for outbound HTTP calls with a timeout to avoid hangs.
var httpClient = &http.Client{Timeout: 10 * time.Second}

func SendPeer(host *HostService) {
    if Configuration.Debug {
        log.Printf("peer-update: preparing to send hosts list to ip=%s", host.Ip)
    }
	//Prepare the request")
	//Post the hosts list to the host
	data, _ := json.Marshal(AllHosts())

	log.Printf("Checking host %v is up\n", host.Ip)
	url := fmt.Sprintf("http://%v/public_info", FormatHttpIpPort_i(host.Ip, host.Port))
    resp, err := httpClient.Get(url)
    if err != nil {
        log.Printf("Failed to contact host %v, %v\n", url, err)
        return
    }
    defer resp.Body.Close()
	//Send our details to the peer
	go func(ip, port string) {
		AnnounceSelf(ip, port)
	}(host.Ip, fmt.Sprintf("%v", host.Port))

	if resp.StatusCode != 200 {
		log.Printf("Failed http request(%v), host %v:%v\n", resp.StatusCode, host.Ip, host.Port)
		return
	}
	//Decode the response into a  info struct
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Printf("Failed to read response body from host %v:%v\n", host.Ip, host.Port)
        return
    }
	var PeerInfo InfoStruct
	err = json.Unmarshal(body, &PeerInfo)
	if err != nil {
		log.Printf("Failed to unmarshal response body from host %v:%v\n", host.Ip, host.Port)
		return
	}
	host.GUID = PeerInfo.GUID
	host.Name = PeerInfo.Name
	host.LastSeen = time.Now()
	host.Services = PeerInfo.Services
    host.Ip = ExtractIpv6(host.Ip)

	prefix := fmt.Sprintf("/frogpond/hosts/%v/", host.Ip)
	ThisNode.SetDataPointWithPrefix_str(prefix, "guid", host.GUID)
	ThisNode.SetDataPointWithPrefix_str(prefix, "name", host.Name)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "port", host.Port)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "lastseen", host.LastSeen)
	ThisNode.SetDataPointWithPrefix_str(prefix, "ip", host.Ip)
	ThisNode.SetDataPointWithPrefix_str(prefix, "path", host.Path)

	prefix = fmt.Sprintf("/frogpond/hosts/%v/", host.GUID)
	ThisNode.SetDataPointWithPrefix_str(prefix, "guid", host.GUID)
	ThisNode.SetDataPointWithPrefix_str(prefix, "name", host.Name)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "port", host.Port)
	ThisNode.SetDataPointWithPrefix_iface(prefix, "lastseen", host.LastSeen)
	ThisNode.SetDataPointWithPrefix_str(prefix, "ip", host.Ip)
	ThisNode.SetDataPointWithPrefix_str(prefix, "path", host.Path)

	log.Printf("Sending hosts list to http://%v/contact", FormatHttpIpPort_i(host.Ip, host.Port))
    resp, err = httpClient.Post(fmt.Sprintf("http://%v/contact", FormatHttpIpPort_i(host.Ip, host.Port)), "application/json", ioutil.NopCloser(strings.NewReader(string(data))))
    if err != nil {
        log.Println("UpdatePeers: Failed to send hosts list to", FormatHttpIpPort_i(host.Ip, host.Port), "err:", err)
    } else {
        defer resp.Body.Close()
        //Read entire body from response
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            log.Println("UpdatePeers: Failed to read response body from", FormatHttpIpPort_i(host.Ip, host.Port), "err:", err)
        } else {
			//log.Printf("Received response from %v - %v", FormatHttpIpPort_i(host.Ip, host.Port), string(body))
			var hostList []*HostService
			err = json.Unmarshal(body, &hostList)
			if err != nil {
				log.Println("UpdatePeers: Failed to unmarshal response body from", FormatHttpIpPort_i(host.Ip, host.Port), "err:", err)
			} else {
                if Configuration.HttpPort == 0 {
                    log.Printf("invalid configuration: HttpPort is 0; defaulting to 16002")
                    Configuration.HttpPort = 16002
                }
                //TODO: We want to reject zro ports but for backwards compatibility we fix them for now
                for _, h := range hostList {
                    if h.Port == 0 {
                        if h.Ports != nil && len(h.Ports) > 0 {
                            h.Port = h.Ports[0]
                        } else {
                            h.Port = Configuration.HttpPort
                        }
                    }
                }
                if Configuration.Debug {
                    log.Printf("peer-update: received hosts list from=%s count=%d", FormatHttpIpPort_i(host.Ip, host.Port), len(hostList))
                }
                AddHosts(hostList)
                AddHost(host)

            }
        }

    }

}

// UpdatePeers sends our host list to all known peers and merges their
// responses into the in-memory host map.
func UpdatePeers() {
    if Configuration.Debug {
        log.Println("peer-update: contacting peers to update peer list")
    }
    for _, host := range Configuration.KnownPeers {
        hostStr := &HostService{GUID: host, Ip: host, Name: "UncontactablePeer", Port: Configuration.HttpPort, Ports: []uint{16002}} //FIXME better guid
        AddHost(hostStr)
        if Configuration.Debug {
            log.Printf("peer-update: added known peer guid=%s host=%s", hostStr.GUID, host)
        }
    }
    for _, host := range AllHosts() {
        SendPeer(host)
    }

}

// Send our data to one peer
func AnnounceSelf(ip, port string) {
	if ThisHost == nil {
		ThisHost = &HostService{}
		ThisHost.GUID = uuid.NewString()
	}
	ThisHost.Ip = ""
	ThisHost.Name = Info.Name
	ThisHost.Services = Info.Services
	ThisHost.Port = Configuration.HttpPort
	ThisHost.Path = "/public_info"

	// Send our info to a peer
	data, _ := json.Marshal(ThisHost)
    resp, err := httpClient.Post(fmt.Sprintf("http://%v/announce", FormatHttpIpPort(ip, port)), "application/json", ioutil.NopCloser(strings.NewReader(string(data))))
    if err != nil {
        log.Println("AnnounceSelf: Failed to send host data to", FormatHttpIpPort(ip, port), "err:", err)
        return
    }
    resp.Body.Close()
    if Configuration.Debug {
        log.Println("peer-update: announced self to", FormatHttpIpPort(ip, port))
    }

}

// AnnounceAll posts this host's public info to all known peers.
func AnnounceAll() {
    for _, host := range AllHosts() {
        if host.Ip == "" {
            continue
        }
        AnnounceSelf(host.Ip, fmt.Sprintf("%v", host.Port))
    }
}

// UpdatePeersData exchanges key/value data with all known peers.
func UpdatePeersData() {
    if Configuration.Debug {
        log.Println("peer-update: contacting peers to update keyval data")
    }

    data := ThisNode.JsonDump()
    for _, host := range AllHosts() {
        //Post the hosts list to the host

        if Configuration.Debug {
            log.Printf("peer-update: sending keyval data to=http://%v/ponds/default", FormatHttpIpPort_i(host.Ip, host.Port))
        }
        resp, err := httpClient.Post(fmt.Sprintf("http://%v/ponds/default", FormatHttpIpPort_i(host.Ip, host.Port)), "application/json", ioutil.NopCloser(strings.NewReader(string(data))))
        if err != nil {
            log.Println("UpdatePeersData: Failed to send keyval data to", FormatHttpIpPort_i(host.Ip, host.Port), "err:", err)
        } else {
            //Read entire body from response
            body, err := ioutil.ReadAll(resp.Body)
            if err != nil {
                log.Println("UpdatePeersData: Failed to read response body from", FormatHttpIpPort_i(host.Ip, host.Port), "err:", err)

            } else {
                d := DataPoolList{}

                err = json.Unmarshal(body, &d)
                if err != nil {
                    log.Println("UpdatePeersData: Failed to unmarshal response body from", FormatHttpIpPort_i(host.Ip, host.Port), "err:", err)
                } else {
                    if Configuration.Debug {
                        log.Printf("peer-update: received keys=%d from=%s", len(d), FormatHttpIpPort_i(host.Ip, host.Port))
                    }
                    ThisNode.applyUpdate(d)
                }
                resp.Body.Close()
            }
        }
    }
}

// Take an ip, port and return a url without trailing slash
func FormatHttpIpPort(ip, port string) string {
	//Count the number of : in ip
	count := strings.Count(ip, ":")
	switch count {
	case 0:
		return ip + ":" + port
	case 1:

		bits := strings.Split(ip, ":")
		//drop last bit
		addr := bits[:len(bits)-1]
		ip = strings.Join(addr, ":")
		return ip + ":" + port
	default:
        if ip[0] == '[' {
            return ip + ":" + port
        }
        return "[" + ip + "]" + ":" + port
    }

}

// As for FormatHttpIpPort, but the port can be a uint
func FormatHttpIpPort_i(ip string, port uint) string {
	return FormatHttpIpPort(ip, fmt.Sprintf("%v", port))
}

// StartServer starts the HTTP server and background peer sync loops.
// Use StartServerWithContext for graceful shutdown via context.
func StartServer(publicport uint) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    StartServerWithContext(ctx, publicport)
}

// StartServerWithContext starts the HTTP server and background loops, and stops when ctx is cancelled.
func StartServerWithContext(ctx context.Context, publicport uint) {

    if Configuration.Debug {
        log.Printf("server: starting with config: %+v", Configuration)
    }
    publicServer := http.NewServeMux()
    //publicServer.HandleFunc("/hello", hello)
    publicServer.HandleFunc("/public_info", public_info)
    publicServer.HandleFunc("/contact", contact)
    publicServer.HandleFunc("/announce", announce)
    publicServer.HandleFunc("/ponds/default", swapdata)

    // Background peer update loop with cancellation support
    go func() {
        ticker := time.NewTicker(time.Second * time.Duration(Configuration.PeerUpdateInterval))
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                if Configuration.Debug {
                    log.Println("server: stopping peer update loop")
                }
                return
            case <-ticker.C:
                AnnounceAll()
                UpdatePeers()
                UpdatePeersData()
            }
        }
    }()

    ThisNode = NewNode()
    srv := &http.Server{Addr: fmt.Sprintf(":%v", publicport), Handler: publicServer}

    // Graceful shutdown on SIGINT/SIGTERM if context not cancelled externally
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        select {
        case <-ctx.Done():
            return
        case <-stop:
            if Configuration.Debug {
                log.Println("server: shutdown signal received")
            }
            shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            _ = srv.Shutdown(shutdownCtx)
        }
    }()

    log.Printf("Frogpond server listening on :%v", publicport)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Println("ListenAndServe: ", err)
    }
}
