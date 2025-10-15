package idrac

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"resty.dev/v3"
)

type Attribute string
type PowerState uint8
type User struct {
	UserName  string `json:"UserName,omitempty"`
	Password  string `json:"Password,omitempty"`
	Enable    string `json:"Enable,omitempty"`
	Privilege Role   `json:"Privilege,omitempty"`
}
type Role string

var (
	RoleAdministrator = Role("511")
	RoleOperator      = Role("499")
	RoleReadonly      = Role("1")
	RoleNone          = Role("0")
)

type cli struct {
	http     *resty.Client
	username string
	password string
	st1, st2 string
}

type dataResult struct {
	PowerState int `xml:"pwState"`
	//Temperature                        any    `xml:"temperatures"`
	BiosVersion                        string `xml:"biosVer"`
	LifecycleControllerFirmwareVersion string `xml:"LCCfwVersion"`
	Sensortype                         struct {
		ThresholdSensorList []struct {
			Sensor []struct {
				Name         string `xml:"name"`
				Reading      int    `xml:"reading"`
				SensorStatus string `xml:"sensorStatus"`
				Units        string `xml:"units"`
			} `xml:"sensor"`
		} `xml:"thresholdSensorList"`
		SensorID           int `xml:"sensorid"`
		DiscreteSensorList struct {
			Sensor []struct {
				SensorStatus string `xml:"sensorStatus"`
				Name         string `xml:"name"`
				Reading      string `xml:"reading"`
			} `xml:"sensor"`
		} `xml:"discreteSensorList"`
	} `xml:"sensortype"`
	EventLogEntries []struct {
		Severity      string `xml:"severity"`
		DateTime      string `xml:"dateTime"`
		DateTimeOrder string `xml:"dateTimeOrder"`
		Description   string `xml:"description"`
	} `xml:"eventLogEntries>eventLogEntry"`
}

func NewClient(host, username, password string) *cli {
	os.Setenv("GODEBUG", "tlsrsakex=1")

	c := &cli{
		http: resty.New().
			SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
			SetContentLength(true).SetBaseURL(host),
		username: username,
		password: password,
	}
	return c
}

func (c *cli) Login() bool {

	var login struct {
		Result int    `xml:"authResult"`
		Furl   string `xml:"forwardUrl"`
		Status string `xml:"status"`
	}

	c.http.R().SetResult(&login).SetBody("user=" + c.username + "&password=" + c.password).Post(c.http.BaseURL() + "/data/login")
	if login.Status != "ok" || login.Result != 0 {
		fmt.Println("login failed")
		return false
	}

	s := regexp.MustCompile(`ST1=(.*),ST2=(.*)`).FindStringSubmatch(login.Furl)
	if len(s) != 3 {
		fmt.Println("login failed")
		return false
	}
	c.st1 = s[1]
	c.st2 = s[2]
	c.http.SetHeader("ST2", c.st2)
	fmt.Println("login seccess")

	return true

}

func (c *cli) Logout() {

	r, _ := c.http.R().Post(c.http.BaseURL() + "/data/logout")
	if r.StatusCode() != 200 {
		fmt.Println("logout failed")
	}
	fmt.Println("logout seccess")
}

func (c *cli) GetData(fields ...string) *dataResult {
	var result dataResult

	r, _ := c.http.R().Post(c.http.BaseURL() + "/data/?get=" + strings.Join(fields, ","))
	fmt.Println(r)
	return &result
}

func (c *cli) GetProcessorInfo() {

	var result struct {
		Processor map[string]struct {
			Name         string `json:"name"`
			Brand        string `json:"brand"`
			CoreCount    int    `json:"core_count"`
			CurrentSpeed int    `json:"current_speed"`
			Version      string `json:"version"`
		} `json:"Processor"`
	}

	c.http.R().SetResult(&result).SetHeader("X_SYSMGMT_OPTIMIZE", "true").Get(c.http.BaseURL() + "/sysmgmt/2012/server/processor")
	for k, v := range result.Processor {

		fmt.Println(k, v.Name, v.Brand, v.CoreCount, v.CurrentSpeed, v.Version)
	}
}

func (c *cli) GetPyDiskInfo() {
	var result struct {
		PDisks map[string]struct {
			Name              string  `json:"name"`
			Size              float64 `json:"size"`
			State             int     `json:"state"`
			Slot              int     `json:"slot"`
			DeviceDescription string  `json:"device_description"`
		} `json:"PDisks"`
	}

	c.http.R().SetResult(&result).SetHeader("X_SYSMGMT_OPTIMIZE", "true").Get(c.http.BaseURL() + "/sysmgmt/2010/storage/pdisk")

	var keys []string
	for k := range result.PDisks {
		if strings.Contains(k, "|P|") {
			continue
		}
		keys = append(keys, k)
	}
	fmt.Println("keys:", keys)
	c.http.R().SetResult(&result).SetHeader("X_SYSMGMT_OPTIMIZE", "true").Get(c.http.BaseURL() + "/sysmgmt/2010/storage/pdisk?keys=" + url.PathEscape("304|C|Disk.Bay.0:Enclosure.Internal.0-1:RAID.Integrated.1-1,304|C|Disk.Bay.1:Enclosure.Internal.0-1:RAID.Integrated.1-1,304|C|Disk.Bay.2:Enclosure.Internal.0-1:RAID.Integrated.1-1,304|C|Disk.Bay.3:Enclosure.Internal.0-1:RAID.Integrated.1-1,304|C|Disk.Bay.4:Enclosure.Internal.0-1:RAID.Integrated.1-1,304|C|Disk.Bay.5:Enclosure.Internal.0-1:RAID.Integrated.1-1,304|C|Disk.Bay.6:Enclosure.Internal.0-1:RAID.Integrated.1-1"))

	for k, v := range result.PDisks {

		fmt.Println(k, v.Slot, v.DeviceDescription, v.State, v.Size)
	}

}
func (c *cli) GetMemoryInfo() {
	var result struct {
		Dimm map[string]struct {
			Size  int `json:"size"`
			Speed int `json:"speed"`
		} `json:"DIMM"`

		Memory struct {
			Capacity       int `json:"capacity"`
			MaxCapacity    int `json:"max_capacity"`
			SlotsAvailable int `json:"slots_available"`
			SlotsUsed      int `json:"slots_used"`
			ErrCorrection  int `json:"err_correction"`
		} `json:"Memory"`
	}
	c.http.R().SetResult(&result).SetHeader("X_SYSMGMT_OPTIMIZE", "true").Get(c.http.BaseURL() + "/sysmgmt/2012/server/memory")
	for k, v := range result.Dimm {
		if v.Size == 0 {
			continue
		}
		fmt.Println(k, v.Size, v.Speed)

	}
	fmt.Println(result.Memory)

}

func (c *cli) FirmwareUpdate(file string) {
	var firmware struct {
		FileDetails []struct {
			Device struct {
				Target string `xml:"target,attr"`
			} `xml:"Device"`
			Message    string `xml:"message,attr"`
			RebootType string `xml:"RebootType,attr"`
		} `xml:"FileDetails"`
	}

	c.http.R().Get(c.http.BaseURL() + "/sysmgmt/2012/server/firmware/queue?splock=1")
	c.http.R().SetResult(&firmware).SetFile("firmwareUpdate", file).Post(c.http.BaseURL() + "/sysmgmt/2012/server/firmware/queue?ST1=" + c.st1)

	fmt.Printf("%+v\n", firmware)
	if !strings.Contains(firmware.FileDetails[0].Message, "Package successfully downloaded.") {
		fmt.Println("upload failed")
		return
	}

	////RebootType="IDRAC <Repository><target>DCIM:INSTALLED#301_C_RAID.Integrated.1-1</target><rebootType>1</rebootType></Repository>
	//r, err = cli.R().SetBody(`<Repository><target>DCIM:INSTALLED#iDRAC.Embedded.1-1#IDRACinfo</target><rebootType>0</rebootType></Repository>`).Put("https://192.168.1.75/sysmgmt/2012/server/firmware")
	//fmt.Println(r, "installFirmware:", err)

}

func (c *cli) PowerOn() {
	c.http.R().Post(c.http.BaseURL() + "/data/?set=pwState:1")

}
func (c *cli) PowerOff() {
	c.http.R().Post(c.http.BaseURL() + "/data/?set=pwState:0")
}

func (c *cli) User(uid int, user User) {
	var idrac struct {
		Users User `json:"iDRAC.Users"`
	}

	if user.Password != "" {
		user.Password = encodeIDRACPassword(user.Password)
	}
	if user.UserName != "" {
		user.UserName = encodeIDRACPassword(user.UserName)
	}

	idrac.Users = user
	u, _ := json.Marshal(idrac)
	us := string(u)
	fmt.Println(us)

	r, err := c.http.R().SetBody(us).Put(c.http.BaseURL() + "/sysmgmt/2012/server/configgroup/iDRAC.Users." + fmt.Sprintf("%d", uid))
	fmt.Println(r, err)
}

func encodeIDRACPassword(plaintext string) string {
	var result strings.Builder

	for _, char := range plaintext {
		result.WriteString(fmt.Sprintf("@%03x", char))
	}

	return result.String()
}
