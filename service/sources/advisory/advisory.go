package advisory

import (
	"fmt"

	"portier/service/sources/domain"
)

const (
	DefaultManagementHost     = "127.0.0.1"
	DefaultManagementPort     = 47831
	RecommendedForwardPortMin = 48000
	RecommendedForwardPortMax = 48999
	PurposeForward            = "forward"
	PurposeManagement         = "management"
)

type CommonPortInfo struct {
	Port     int
	Label    string
	Category string
	Severity string
}

var CommonPorts = []CommonPortInfo{
	{Port: 20, Label: "FTP data", Category: "network", Severity: "danger"},
	{Port: 21, Label: "FTP control", Category: "network", Severity: "danger"},
	{Port: 22, Label: "SSH", Category: "system", Severity: "danger"},
	{Port: 25, Label: "SMTP", Category: "network", Severity: "danger"},
	{Port: 53, Label: "DNS", Category: "network", Severity: "danger"},
	{Port: 80, Label: "HTTP", Category: "network", Severity: "danger"},
	{Port: 110, Label: "POP3", Category: "network", Severity: "danger"},
	{Port: 123, Label: "NTP", Category: "network", Severity: "danger"},
	{Port: 143, Label: "IMAP", Category: "network", Severity: "danger"},
	{Port: 443, Label: "HTTPS", Category: "network", Severity: "danger"},
	{Port: 445, Label: "SMB / Windows file sharing", Category: "system", Severity: "danger"},
	{Port: 3000, Label: "React / Next.js / common dev server", Category: "dev", Severity: "warning"},
	{Port: 3001, Label: "Common secondary dev server", Category: "dev", Severity: "warning"},
	{Port: 3306, Label: "MySQL / MariaDB", Category: "database", Severity: "warning"},
	{Port: 3389, Label: "Remote Desktop", Category: "system", Severity: "danger"},
	{Port: 4200, Label: "Angular dev server", Category: "dev", Severity: "warning"},
	{Port: 5000, Label: "Flask / common dev API", Category: "dev", Severity: "warning"},
	{Port: 5173, Label: "Vite dev server", Category: "dev", Severity: "warning"},
	{Port: 5432, Label: "PostgreSQL", Category: "database", Severity: "warning"},
	{Port: 6379, Label: "Redis", Category: "database", Severity: "warning"},
	{Port: 8000, Label: "Common dev HTTP server", Category: "dev", Severity: "warning"},
	{Port: 8080, Label: "Alternate HTTP / proxy / dev server", Category: "dev", Severity: "warning"},
	{Port: 8443, Label: "Alternate HTTPS", Category: "network", Severity: "warning"},
	{Port: 9229, Label: "Node.js debugger", Category: "debug", Severity: "danger"},
	{Port: 27017, Label: "MongoDB", Category: "database", Severity: "warning"},
}

type Input struct {
	Port       int
	ListenHost string
	Purpose    string
}

func GetCommonPortInfo(port int) (CommonPortInfo, bool) {
	for _, info := range CommonPorts {
		if info.Port == port {
			return info, true
		}
	}
	return CommonPortInfo{}, false
}

func IsRecommendedForwardPort(port int) bool {
	return port >= RecommendedForwardPortMin && port <= RecommendedForwardPortMax
}

func GetPortAdvisories(input Input) []domain.PortAdvisory {
	advisories := make([]domain.PortAdvisory, 0)

	if commonPort, ok := GetCommonPortInfo(input.Port); ok {
		advisories = append(advisories, domain.PortAdvisory{
			Code:     "COMMON_PORT",
			Severity: commonPort.Severity,
			Message:  fmt.Sprintf("Port %d is commonly used by %s.", input.Port, commonPort.Label),
		})
	}

	if input.Port < 1024 {
		advisories = append(advisories, domain.PortAdvisory{
			Code:     "PRIVILEGED_PORT",
			Severity: "danger",
			Message:  fmt.Sprintf("Port %d is privileged and may require elevated permissions.", input.Port),
		})
	}

	if input.Purpose == PurposeForward && !IsRecommendedForwardPort(input.Port) {
		advisories = append(advisories, domain.PortAdvisory{
			Code:     "OUTSIDE_RECOMMENDED_RANGE",
			Severity: "info",
			Message: fmt.Sprintf(
				"Port %d is outside Portier's recommended forwarding range %d-%d.",
				input.Port,
				RecommendedForwardPortMin,
				RecommendedForwardPortMax,
			),
		})
	}

	if input.ListenHost == "0.0.0.0" && input.Purpose == PurposeForward {
		advisories = append(advisories, domain.PortAdvisory{
			Code:     "LAN_EXPOSURE",
			Severity: "warning",
			Message:  "Listening on 0.0.0.0 exposes this forwarded port on the LAN.",
		})
	}

	if input.ListenHost == "0.0.0.0" && input.Purpose == PurposeManagement {
		advisories = append(advisories, domain.PortAdvisory{
			Code:     "MANAGEMENT_LAN_EXPOSURE",
			Severity: "danger",
			Message:  "Listening on 0.0.0.0 exposes the Portier management UI/API on the LAN.",
		})
	}

	return advisories
}
