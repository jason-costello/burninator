package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/lib/pq"
	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
	"net/http"
	"os"
	"strconv"
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

type Trogdor struct {
	twilioClient *twilio.RestClient
	db           *sql.DB
	config       config
}

type config struct {
	TwilioAcctSID     string
	TwilioMsgSID      string
	TwilioAuthToken   string
	TwilioPhoneNumber string
	BurnBanURL        string
	DBUser            string
	DBPass            string
	DBHost            string
	DBPort            int
	DBName            string
	DBSSLMode         string
	ToAddresses       []string
	PollingInterval   time.Duration
}

func NewClient() *Trogdor {
	conf := getConfig()
	tc := twilio.NewRestClientWithParams(
		twilio.ClientParams{
			Username: conf.TwilioAcctSID,
			Password: conf.TwilioAuthToken,
		})

	return &Trogdor{
		twilioClient: tc,
		db:           nil,
		config:       config{},
	}

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

	conf.DBUser = os.Getenv("DBUSER")
	conf.DBPass = os.Getenv("DBPASS")
	conf.DBHost = os.Getenv("DBHOST")
	dbp, err := strconv.Atoi(os.Getenv("DBPORT"))
	if err != nil {
		panic(err)
	}
	conf.DBPort = dbp
	conf.DBName = os.Getenv("DBNAME")
	conf.DBSSLMode = os.Getenv("DBSSLMODE")

	return conf
}

func main() {

	b := NewClient()
	dbConnStr := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		b.config.DBHost, b.config.DBPort, b.config.DBUser, b.config.DBPass, b.config.DBName)

	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	c := time.Tick(b.config.PollingInterval)
	for _ = range c {
		status, err := GetBurnBanStatus(b.config.BurnBanURL)
		if err != nil {
			fmt.Println("Error processing, default to ON")
		}
		previousStatus, err := b.readPreviousStatus()
		if err != nil {
			previousStatus = ON
		}

		if previousStatus != status {
			if err := b.writeStatus(status); err != nil {
				fmt.Println("error returned from writing status: ", err)
			}
			if err := b.sendNotification(b.config, status); err != nil {
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

func (t *Trogdor) writeStatus(status BurnBanStatus) error {

	_, err := t.db.Exec("Insert into burn_ban(status, created_at) values(?, ?)", status.String(), time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (t *Trogdor) readPreviousStatus() (BurnBanStatus, error) {
	row := t.db.QueryRow("Select status from burn_ban order by created_at desc limit 1;")
	if row == nil {
		return ON, errors.New("no data found in db for previous status")
	}
	var status string

	if err := row.Scan(&status); err != nil {
		return ON, err
	}

	return BurnBanStatus(status), nil

}

func (b *Trogdor) sendNotification(conf config, status BurnBanStatus) error {

	params := &openapi.CreateMessageParams{}
	params.SetFrom(conf.TwilioPhoneNumber)
	params.SetBody("Burn Ban is " + status.String())

	for _, addr := range conf.ToAddresses {
		time.Sleep(1 * time.Second)
		params.SetTo(addr)
		_, err := b.twilioClient.Api.CreateMessage(params)
		if err != nil {
			fmt.Printf("%v: %q failed to send: %s\n", time.Now().String(), fmt.Sprintf("Burn Ban is %s", status.String()), err.Error())
			return err
		}
		fmt.Printf("%v: %q sent\n", time.Now().String(), fmt.Sprintf("Burn Ban is %s", status.String()))
	}
	return nil
}
