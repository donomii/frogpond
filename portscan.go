package frogpond

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/donomii/goof"
	"github.com/google/uuid"
	"github.com/mostlygeek/arp"
	"golang.org/x/sync/semaphore"
)

//https://gist.github.com/kotakanbe/d3059af990252ba89a82

type PortScanner struct {
	ip   string
	lock *semaphore.Weighted
}

func Ulimit() int64 {

	return 1
}

var GlobalScanSemaphore = semaphore.NewWeighted(5)

var PortsToScan = []uint{16002}

func scanPorts() []uint {
	sp := PortsToScan
	return sp
}

var AppendHostsLock = &sync.Mutex{}

func AppendHosts(hs []*HostService) {
	AddHosts(hs)
}
func ArpScan() {

	keys := []string{}
	for k, _ := range arp.Table() {
		keys = append(keys, k)
	}
	log.Printf("Scanning %v\n", keys)
	found := scanIps(keys, scanPorts())
	AddHosts(found)

}
func ScanC() {

	ips := goof.AllIps()

	classB := map[string]bool{}
	for _, ip := range ips {
		found := ScanNetwork(ip+"/24", scanPorts())
		AddHosts(found)
		bits := strings.Split(ip, ".")
		b := bits[0] + "." + bits[1] + ".0.0"
		classB[b] = true
	}
}

func ScanConfig() {
	networks := Configuration.Networks
	log.Println("Scanning user defined networks:", networks)
	for _, network := range networks {
		log.Println("Scanning user defined network:", network)
		found := ScanNetwork(network, scanPorts())
		AddHosts(found)
	}

}


func ScanHostPublicInfo(host *HostService) {
    url := fmt.Sprintf("http://%v:%v/public_info", host.Ip, host.Port)
    //fmt.Println("Public info url:", url)
    resp, err := httpClient.Get(url)
    if err == nil {
        log.Println("Got response")
        body, err := ioutil.ReadAll(resp.Body)
        resp.Body.Close()
        if err == nil {
            log.Println("Got body")
            var s InfoStruct
            err := json.Unmarshal(body, &s)
            if err == nil {
                log.Printf("Unmarshalled body %v", s)
				host.Services = s.Services
				host.Name = s.Name
				host.LastSeen = time.Now()
				fmt.Printf("Updated host details for %v, at IP %v\n", host.Name, host.Ip)
			}
		}
	}
}

func ScanAllHostsPublicInfo() {
    for _, v := range AllHosts() {
        url := fmt.Sprintf("http://%v:%v/public_info", v.Ip, v.Port)
        resp, err := httpClient.Get(url)
        if err == nil {
            log.Println("Got response")
            body, err := ioutil.ReadAll(resp.Body)
            resp.Body.Close()
            if err == nil {
				log.Println("Got body")
				var s InfoStruct
				err := json.Unmarshal(body, &s)
				if err == nil {
					log.Printf("Unmarshalled body %v", s)
					v.Services = s.Services
					v.Name = s.Name
					v.LastSeen = time.Now()
					fmt.Printf("Updated host details for %v, at IP %v\n", v.Name, v.Ip)
				} else {
					log.Printf("Error unmarshalling body %v, %v", err, body)
				}
			} else {
				log.Printf("Error reading body %v", err)

			}
		} else {
			log.Printf("Error getting response %v", err)
		}
	}

}

func ScanPort(ip string, port uint, timeout time.Duration) bool {
	if port == 0 {
		return false
	}
	target := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", target, timeout)

	if err != nil {
		//log.Println(err)
		if strings.Contains(err.Error(), "open files") || strings.Contains(err.Error(), "requested address") {
			time.Sleep(timeout)
			//fmt.Println("Sequential Scanner: ",ip, ":", port, "retry :", err.Error())
			ScanPort(ip, port, timeout)
		} else {
			//fmt.Println("Sequential Scanner: ",ip, ":", port, "closed :", err.Error())
		}
		return false
	}

	conn.Close()
	//AddNetworkNode(ip, "http", port)
	//fmt.Println(ip, port, "open")
	return true
}

func (ps *PortScanner) Start(startPort, endPort uint, timeout time.Duration) {
	wg := sync.WaitGroup{}
	defer wg.Wait()

	for port := startPort; port <= endPort; port++ {
		ps.lock.Acquire(context.TODO(), 1)
		wg.Add(1)
		go func(port uint) {
			defer ps.lock.Release(1)
			defer wg.Done()
			ScanPort(ps.ip, port, timeout)
		}(port)
	}
}

func (ps *PortScanner) ScanList(timeout time.Duration, ports []uint) (out []uint) {
    wg := sync.WaitGroup{}
    results := make(chan uint, len(ports))

    for _, port := range ports {
        ps.lock.Acquire(context.TODO(), 1)
        wg.Add(1)
        go func(port uint) {
            defer ps.lock.Release(1)
            defer wg.Done()
            if ScanPort(ps.ip, port, timeout) {
                results <- port
            }
        }(port)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    for p := range results {
        out = append(out, p)
    }
    return out
}

// http://play.golang.org/p/m8TNTtygK0
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func CidrHosts(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// remove network address and broadcast address
	lenIPs := len(ips)
	switch {
	case lenIPs < 2:
		return ips, nil

	default:
		return ips[1 : len(ips)-1], nil
	}
}

func ScanNetwork(cidr string, ports []uint) (out []*HostService) {
    return []*HostService{} //FIXME
    var wg sync.WaitGroup
    hosts, _ := CidrHosts(cidr)
    fmt.Println("Started sequential network scan on", cidr, "for ports", ports, "on", len(hosts), "hosts", hosts)
    for _, v := range hosts {
		wg.Add(1)

		ps := &PortScanner{
			ip:   v,
			lock: GlobalScanSemaphore,
		}
		go func(v string) {
			//log.Printf("Scanning %v...\n", v)
            openPorts := ps.ScanList(5000*time.Millisecond, ports)
            if len(openPorts) > 0 {
                fmt.Printf("Found %v open ports on %v\n", openPorts, v)
                AppendHostsLock.Lock()
                out = append(out, &HostService{uuid.NewString(), v, openPorts, Configuration.HttpPort, nil, "", time.Now(), "unknown"})
                AppendHostsLock.Unlock()
                AddHost(&HostService{uuid.NewString(), v, openPorts, Configuration.HttpPort, nil, "", time.Now(), "unknown"})
                UpdatePeers()
            }
            wg.Done()
            //log.Printf("Scanned %v!\n", v)
        }(v)
    }
    wg.Wait()
    return out
}

func scanIps(hosts []string, ports []uint) (out []*HostService) {
    var wg sync.WaitGroup
    results := make(chan *HostService, len(hosts))

    for _, v := range hosts {
        wg.Add(1)
        //fmt.Println("Scanning", v)
        ps := &PortScanner{
            ip:   v,
            lock: GlobalScanSemaphore,
        }
        go func(v string) {
            openPorts := ps.ScanList(3000*time.Millisecond, ports)
            if len(openPorts) > 0 {
                results <- &HostService{uuid.NewString(), v, openPorts, Configuration.HttpPort, nil, "", time.Time{}, "unknown"}
            }
            wg.Done()
        }(v)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    for h := range results {
        out = append(out, h)
    }
    return out
}
