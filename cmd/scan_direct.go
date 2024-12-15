package cmd

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Ayanrajpoot10/bugscanx-go/pkg/queuescanner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// scanDirectCmd represents the scanDirect command
var scanDirectCmd = &cobra.Command{
	Use:   "direct",
	Short: "Scan using direct connection",
	Run:   scanDirectRun,
}

var (
	scanDirectFlagFilename   string
	scanDirectFlagServerList string
	scanDirectFlagHttps      bool
	scanDirectFlagTimeout    int
	scanDirectFlagOutput     string
	scanDirectFlagMethod     string
	scanDirectFlagBothSchemes bool
)

func init() {
	scanCmd.AddCommand(scanDirectCmd)

	scanDirectCmd.Flags().StringVarP(&scanDirectFlagFilename, "filename", "f", "", "domain list filename")
	scanDirectCmd.Flags().StringVarP(&scanDirectFlagServerList, "server-list", "s", "all", "server list")
	scanDirectCmd.Flags().BoolVar(&scanDirectFlagHttps, "https", false, "use https")
	scanDirectCmd.Flags().IntVar(&scanDirectFlagTimeout, "timeout", 3, "connect timeout")
	scanDirectCmd.Flags().StringVarP(&scanDirectFlagOutput, "output", "o", "", "output result")
	scanDirectCmd.Flags().StringVar(&scanDirectFlagMethod, "method", "HEAD", "HTTP method to use (e.g., GET, POST, etc.)")
	scanDirectCmd.Flags().BoolVar(&scanDirectFlagBothSchemes, "both-schemes", false, "scan both HTTP and HTTPS")

	scanDirectCmd.MarkFlagFilename("filename")
	scanDirectCmd.MarkFlagRequired("filename")
}

type scanDirectRequest struct {
	Domain     string
	Https      bool
	ServerList []string
}

type scanDirectResponse struct {
	Color      *color.Color
	Request    *scanDirectRequest
	NetIPList  []net.IP
	StatusCode int
	Server     string
	Location   string
}

var httpClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
	Timeout: 10 * time.Second,
}

var ctxBackground = context.Background()

func scanDirect(c *queuescanner.Ctx, p *queuescanner.QueueScannerScanParams) {
	req := p.Data.(*scanDirectRequest)

	ctxTimeout, cancel := context.WithTimeout(ctxBackground, 3*time.Second)
	defer cancel()
	netIPList, err := net.DefaultResolver.LookupIP(ctxTimeout, "ip4", req.Domain)
	if err != nil {
		return
	}
	ip := netIPList[0].String()

	schemes := []string{"http"}
	if req.Https || scanDirectFlagBothSchemes {
		schemes = append(schemes, "https")
	}
	for _, scheme := range schemes {
		method := strings.ToUpper(scanDirectFlagMethod)
		httpReq, err := http.NewRequest(method, fmt.Sprintf("%s://%s", scheme, req.Domain), nil)
		if err != nil {
			continue
		}

		httpRes, err := httpClient.Do(httpReq)
		if err != nil {
			continue
		}

		hServer := httpRes.Header.Get("Server")
		hServerLower := strings.ToLower(hServer)
		hCfRay := httpRes.Header.Get("CF-RAY")
		hLocation := httpRes.Header.Get("Location")

		if hLocation == "https://jio.com/BalanceExhaust" {
			return
		}

		var resColor *color.Color

		isHiddenCloudflare := slices.Contains(req.ServerList, "cloudflare") && hCfRay != "" && hServerLower != "cloudflare"

		if slices.Contains(req.ServerList, hServerLower) || isHiddenCloudflare {
			if isHiddenCloudflare {
				resColor = colorG1
				hServer = fmt.Sprintf("%s (cf)", hServer)
			} else {
				switch hServerLower {
				case "cloudflare":
					resColor = colorG1
				case "akamaighost":
					resColor = colorY1
				case "akamai":
					resColor = colorY1
				case "cloudfront":
					resColor = colorC1
				case "amazons3":
					resColor = colorC1
				case "apache":
					resColor = colorW1
				case "nginx":
					resColor = colorW1
				case "varnish":
					resColor = colorM1
				case "fastly":
					resColor = colorM1
				case "microsoft":
					resColor = colorC2
				case "azure":
					resColor = colorC2
				case "cachefly":
					resColor = colorY2
				case "Alibaba":
					resColor = colorY2
				case "Tencent":
					resColor = colorM2
				default:
					resColor = colorB1
					hServer = fmt.Sprintf("%s others", hServer)
				}
				if len(req.ServerList) == 1 {
					resColor = colorG1
				}
			}
			res := &scanDirectResponse{
				Color:      resColor,
				Request:    req,
				NetIPList:  netIPList,
				StatusCode: httpRes.StatusCode,
				Server:     hServer,
				Location:   hLocation,
			}
			c.ScanSuccess(res, nil)
		} else {
			resColor = colorB1
			res := &scanDirectResponse{
				Color:      resColor,
				Request:    req,
				NetIPList:  netIPList,
				StatusCode: httpRes.StatusCode,
				Server:     "others",
				Location:   hLocation,
			}
			c.ScanSuccess(res, nil)
		}

		if hLocation != "" {
			hLocation = fmt.Sprintf(" -> %s", hLocation)
		}

		s := fmt.Sprintf(
			"%-15s  %-3d  %-16s    %s%s",
			ip,
			httpRes.StatusCode,
			hServer,
			req.Domain,
			hLocation,
		)

		s = resColor.Sprint(s)

		c.Log(s)
	}

}

func scanDirectRun(cmd *cobra.Command, args []string) {
	domainList := make(map[string]bool)
	domainListFile, err := os.Open(scanDirectFlagFilename)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer domainListFile.Close()

	scanner := bufio.NewScanner(domainListFile)
	for scanner.Scan() {
		domain := scanner.Text()
		domainList[domain] = true
	}

	var serverList []string

	scanDirectFlagServerListLower := strings.ToLower(scanDirectFlagServerList)

	if scanDirectFlagServerListLower == "all" {
		serverList = []string{
			"cloudflare",
			"cloudfront",
			"akamaighost",
			"akamai",
			"amazons3",
			"apache",
			"nginx",
			"varnish",
			"fastly",
			"microsoft",
			"azure",
			"cachefly",
			"alibaba",
			"tencent",
		}
	} else {
		serverList = strings.Split(scanDirectFlagServerListLower, ",")
	}

	queueScanner := queuescanner.NewQueueScanner(scanFlagThreads, scanDirect)

	colorC1.Printf("\n%-15s  ", "IP")
	colorY1.Printf("%-3s  ", "CODE")
	colorM1.Printf("%-16s    ", "SERVER")
	colorG1.Printf("%-20s\n", "HOST")
	colorW1.Printf("%-15s  %-3s  %-16s    %-20s\n", "----", "----", "------", "----")

	for domain := range domainList {
		queueScanner.Add(&queuescanner.QueueScannerScanParams{
			Name: domain,
			Data: &scanDirectRequest{
				Domain:     domain,
				Https:      scanDirectFlagHttps,
				ServerList: serverList,
			},
		})
	}
	queueScanner.Start(func(c *queuescanner.Ctx) {
		if len(c.ScanSuccessList) == 0 {
			return
		}

		c.Log("")

		mapServerList := make(map[string][]*scanDirectResponse)

		for _, data := range c.ScanSuccessList {
			res, ok := data.(*scanDirectResponse)
			if !ok {
				continue
			}

			serverLower := strings.ToLower(res.Server)

			if res.Server == "others" || !slices.Contains(serverList, serverLower) {
				mapServerList["others"] = append(mapServerList["others"], res)
			} else {
				mapServerList[res.Server] = append(mapServerList[res.Server], res)
			}
		}

		domainList := make([]string, 0)
		ipList := make([]string, 0)

		for server, resList := range mapServerList {
			if len(resList) == 0 {
				continue
			}

			var resColor *color.Color

			mapIPList := make(map[string]bool)
			mapDomainList := make(map[string]bool)

			for _, res := range resList {
				if resColor == nil {
					resColor = res.Color
				}

				for _, netIP := range res.NetIPList {
					ip := netIP.String()
					mapIPList[ip] = true
				}

				mapDomainList[res.Request.Domain] = true
			}

			if server == "others" {
				resColor = colorB1
			}

			c.Log(resColor.Sprintf("\n%s\n", server))

			domainList = append(domainList, fmt.Sprintf("# %s", server))
			for domain := range mapDomainList {
				domainList = append(domainList, domain)
				c.Log(resColor.Sprint(domain))
			}
			domainList = append(domainList, "")
			c.Log("")

			ipList = append(ipList, fmt.Sprintf("# %s", server))
			for ip := range mapIPList {
				ipList = append(ipList, ip)
				c.Log(resColor.Sprint(ip))
			}
			ipList = append(ipList, "")
			c.Log("")
		}

		outputList := make([]string, 0)
		outputList = append(outputList, domainList...)
		outputList = append(outputList, ipList...)

		if scanDirectFlagOutput != "" {
			err := os.WriteFile(scanDirectFlagOutput, []byte(strings.Join(outputList, "\n")), 0644)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			fmt.Print(colorG1.Sprintf("✅ Results saved to %s\n", scanDirectFlagOutput))
		}
	})
}
