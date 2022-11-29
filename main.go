package main

import (
	"bufio"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type BurnBanStatus string

func (b BurnBanStatus) String() string {
	if b == OFF {
		return "Off"
	}
	return "On"
}

const (
	OFF BurnBanStatus = "Off"
	ON  BurnBanStatus = "On"
)

type config struct {
	TwilioAcctSID     string
	TwilioMsgSID      string
	TwilioAuthToken   string
	TwilioPhoneNumber string
	BurnBanURL        string
	ToAddresses       []string
	PollingInterval   time.Duration
}

func getConfig() config {
	var conf config
	conf.TwilioAcctSID = os.Getenv("TWILIO_ACCOUNT_SID")
	conf.TwilioAuthToken = os.Getenv("TWILIO_AUTH_TOKEN")
	conf.TwilioPhoneNumber = os.Getenv("TWILIO_PHONE_NUMBER")
	conf.BurnBanURL = os.Getenv("BBURL")
	addrs := os.Getenv("TO_ADDRESSES")
	conf.ToAddresses = strings.Split(addrs, ",")

	timeDuration, err := time.ParseDuration(os.Getenv("POLLING_INTERVAL"))
	if err != nil {
		timeDuration = 10 * time.Minute
	}
	conf.PollingInterval = timeDuration
	if conf.TwilioPhoneNumber == "" || conf.TwilioAuthToken == "" || conf.TwilioAcctSID == "" || conf.BurnBanURL == "" || len(conf.ToAddresses) < 1 {
		panic(fmt.Sprintf("config not complete, has empty values: %#+v\n", conf))
	}
	return conf
}

func main() {
	conf := getConfig()
	fp := "bbstatus_ON.txt"

	c := time.Tick(conf.PollingInterval)
	for _ = range c {
		status, err := GetBurnBanStatus(conf.BurnBanURL)
		if err != nil {
			fmt.Println("Error processing, default to ON")
		}
		previousStatus, err := readPreviousStatus(fp)
		if err != nil {
			previousStatus = ON
		}

		if previousStatus != status {
			if err := writeStatus(fp, status); err != nil {
				fmt.Println("error returned from writing status: ", err)
			}
			if err := sendNotification(conf, status); err != nil {
				fmt.Println("error sending notifications: ", err)
			}
			fmt.Println("Status Changed, send notification")
		}

	}
}

func GetBurnBanStatus(url string) (BurnBanStatus, error) {
	doc, err := getPage(http.DefaultClient, url)
	if err != nil {
		return OFF, err
	}
	return getBurnBanStatus(doc), nil

}

func getPage(client *http.Client, url string) (*goquery.Document, error) {
	resp, err := client.Get(url)
	if err != nil {
		return &goquery.Document{}, err
	}
	defer resp.Body.Close()
	return goquery.NewDocumentFromReader(resp.Body)
}

func getBurnBanStatus(doc *goquery.Document) BurnBanStatus {
	var status = ON
	doc.Find("ul.style1:nth-child(2) > li").Each(func(i int, li *goquery.Selection) {
		liLower := strings.ToLower(li.Text())
		if strings.Contains(liLower, "burn ban") {
			if strings.Contains(liLower, "off") {
				status = OFF
			}
		}
	})
	return status
}

func writeStatus(filePath string, status BurnBanStatus) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	tm := time.Now().Format(time.RFC3339)
	statusStr := status.String()
	if statusStr == "On" {
		statusStr = "On "
	}

	payload := fmt.Sprintf("%s::%s\n", statusStr, tm)

	if _, err = f.WriteString(payload); err != nil {
		return err
	}
	return nil
}

func readPreviousStatus(filePath string) (BurnBanStatus, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return ON, err
	}
	defer f.Close()

	previousStatus := ON
	bytesPerLine := 31
	reader := bufio.NewReader(f)
	buffer := make([]byte, bytesPerLine)

	lineCount := 0
	byteCount := 0

	for {
		_, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return ON, err
		}
		lineCount++
		byteCount += bytesPerLine
	}
	ret, err := f.Seek(0, 2)
	ret = ret - int64(bytesPerLine)
	if err != nil {
		fmt.Println("error seeking: ", err)
		return ON, err
	}
	if _, err := f.ReadAt(buffer, ret); err != nil {
		return ON, err
	}
	line := fmt.Sprint(strings.TrimSpace(string(buffer)))
	rawStatus := strings.Split(line, "::")[0]
	line = strings.TrimSpace(rawStatus)

	previousStatusStr := strings.ToUpper(line)
	if strings.ToUpper(previousStatusStr) == strings.ToUpper(OFF.String()) {
		previousStatus = OFF
	} else {
		previousStatus = ON
	}
	return previousStatus, nil
}

func sendNotification(conf config, status BurnBanStatus) error {
	client := twilio.NewRestClientWithParams(
		twilio.ClientParams{
			Username: conf.TwilioAcctSID,
			Password: conf.TwilioAuthToken,
		})
	params := &openapi.CreateMessageParams{}
	params.SetFrom(conf.TwilioPhoneNumber)
	params.SetBody("Burn Ban is " + status.String())

	for _, addr := range conf.ToAddresses {
		time.Sleep(1 * time.Second)
		params.SetTo(addr)
		_, err := client.Api.CreateMessage(params)
		if err != nil {
			fmt.Printf("%v: %q failed to send: %s\n", time.Now().String(), fmt.Sprintf("Burn Ban is %s", status.String()), err.Error())
			return err
		}
		fmt.Printf("%v: %q sent\n", time.Now().String(), fmt.Sprintf("Burn Ban is %s", status.String()))
	}
	return nil
}
