package logging

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

type Severity int

func (s Severity) String() string {
	return strconv.Itoa(int(s))
}

type Facility int

func (f Facility) String() string {
	return strconv.Itoa(int(f))
}

type RFCVersion string

const (
	Emergency Severity = iota
	Alert
	Critical
	Error
	Warning
	Notice
	Informational
	Debug
)

const (
	KernelMessages Facility = iota
	UserLevelMessages
	MailSystem
	SystemDaemons
	AuthMessages
	InternalSyslogMessage
	LinePrinterSubsystem
	NetworkNewsSubsystem
	UUCPSubsystem
	ClockDaemon
	// Yeah, there are 2 codes with the same name
	AuthMessages2
	FTPDaemon
	NTPSubsystem
	LogAudit
	LogAlert
	ClockDaemon2
	LocalUse0
	LocalUse1
	LocalUse2
	LocalUse3
	LocalUse4
	LocalUse5
	LocalUse6
	LocalUse7
)

const (
	RFC5424 RFCVersion = "rfc5424"
	RFC3164 RFCVersion = "rfc3164"
)

const (
	DefaultSeverityLevel = Informational
)

type LogMessage struct {
	Timestamp  time.Time
	Hostname   string
	Priority   int
	Facility   Facility
	Severity   Severity
	BinaryName string
	ProcID     int
	Message    string
	RFC        RFCVersion
}

func validateMessage(msg map[string]interface{}, rfc RFCVersion) bool {
	fields := []string{
		"timestamp", "hostname", "priority",
		"facility", "severity",
	}
	if rfc == RFC5424 {
		fields = append(fields, []string{
			"app_name", "version", "proc_id",
			"msg_id", "structured_data", "message"}...)
	} else if rfc == RFC3164 {
		fields = append(fields, []string{"tag", "content"}...)
	}

	for _, val := range fields {
		if _, ok := msg[val]; !ok {
			return false
		}
	}
	return true
}

func getRFCVersion(msg map[string]interface{}) (RFCVersion, error) {
	var rfc RFCVersion
	if _, ok := msg["structured_data"]; ok {
		rfc = RFC5424
	} else if _, ok := msg["content"]; ok {
		rfc = RFC3164
	} else {
		return rfc, fmt.Errorf("invalid syslog message")
	}
	if !validateMessage(msg, rfc) {
		return rfc, fmt.Errorf("invalid syslog message")
	}
	return rfc, nil
}

func SyslogToLogMessage(msg map[string]interface{}) (LogMessage, error) {
	rfc, err := getRFCVersion(msg)
	if err != nil {
		return LogMessage{}, errors.Wrap(err, "getting RFC version")
	}
	switch rfc {
	case RFC3164:
		return LogMessage{
			Timestamp:  msg["timestamp"].(time.Time),
			Hostname:   msg["hostname"].(string),
			Priority:   msg["priority"].(int),
			Facility:   Facility(msg["facility"].(int)),
			Severity:   Severity(msg["severity"].(int)),
			BinaryName: msg["tag"].(string),
			Message:    msg["content"].(string),
			RFC:        rfc,
		}, nil
	case RFC5424:
		var procID int
		parsedProcID := msg["proc_id"].(string)
		if parsedProcID != "" && parsedProcID != "-" {
			procID, _ = strconv.Atoi(parsedProcID)
		}
		return LogMessage{
			Timestamp:  msg["timestamp"].(time.Time),
			Hostname:   msg["hostname"].(string),
			Priority:   msg["priority"].(int),
			Facility:   Facility(msg["facility"].(int)),
			Severity:   Severity(msg["severity"].(int)),
			BinaryName: msg["app_name"].(string),
			Message:    msg["message"].(string),
			ProcID:     procID,
			RFC:        rfc,
		}, nil
	default:
		return LogMessage{}, fmt.Errorf("failed to parse log message")
	}
}
