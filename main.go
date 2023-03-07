package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type sms struct {
	M string `json:"m"`
}

const txDebit = "Debit"
const txCredit = "Credit"

type message struct {
	TxType   string    `json:"type"`
	Bank     string    `json:"bank"`
	Account  string    `json:"account"`
	Amount   float64   `json:"amount"`
	DateTime time.Time `json:"time"`
}

var messages []message

var patterns = []*regexp.Regexp{
	// Spent Card no. XX0000 INR 1579 13-12-22 19:57:25 ABCDEFG IND Avl Lmt INR 123456.05 SMS BLOCK 1696 to 918691000002, if not you - Axis Bank
	regexp.MustCompile(`(?m)(?P<txtype>\w+) Card no. (?P<account>\w+) INR (?P<amount>[\d,.]+) (?P<date>[\d-]+) (?P<time>[\d:]+) (?P<receiver>[\w\s]+) Avl Lmt INR (?P<balance>[\d,.]+)`),

	// INR 232.42 spent on ICICI Bank Card XX0000 on 04-Mar-23 at ONE97 COMMUNICA. Avl Lmt: INR 1,12,456.28. To dispute,call 18002662/SMS BLOCK 0000 to 9215676766
	regexp.MustCompile(`(?m)INR (?P<amount>[\d,.]+) (?P<txtype>\w+) on ICICI Bank Card (?P<account>\w+) on (?P<date>[\w-]+) at (?P<receiver>[\w\s]+). Avl Lmt: INR (?P<balance>[\d,]+.?\d{2}?)`),
}

var debitStrMap = map[string]bool{
	"Spent":    true,
	"spent":    true,
	"debit":    true,
	"debited":  true,
	"from A/c": true}

// (\d{2}-\w{3}-\d{2})|(\d{2}-\d{2}-\d{4})|(\d{4}-\d{2}-\d{2})|(\d{2}/\d{2}/\d{2})
var dateRe = regexp.MustCompile(`(?m)(\d{2}-\w{3}-\d{2})|(\d{2}-\d{2}-\d{4})|(\d{4}-\d{2}-\d{2})|(\d{2}/\d{2}/\d{2})|(\d{2}-\w{3}-\d{4})|(\d{2}-\d{2}-\d{2})`)

// \d{2}:\d{2}
var timeRe = regexp.MustCompile(`(?m)\d{2}:\d{2}`)

// ICICI|Federal|SBI
var bankRe = regexp.MustCompile(`(?m)ICICI|Federal|SBI|HDFC|Citi|Axis`)

func main() {
	router := gin.Default()

	router.GET("v1/messages", getMessages)
	router.POST("v1/messages", postMessage)

	err := router.Run("localhost:8080")
	if err != nil {
		panic(err)
	}
}

func getMessages(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, messages)
}

func parseAmount(m string) (float64, error) {
	amountStr := strings.Replace(m, ",", "", -1)
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		log.Println(err)
		return 0, fmt.Errorf("parseAmount error: %v", err)
	}
	return amount, nil
}

func parseDateTime(dt string, tm string) (time.Time, error) {
	var t time.Time

	dateStr := dateRe.FindStringSubmatch(dt)
	if dateStr == nil {
		return t, errors.New("no date string found")
	}
	tStr := timeRe.FindString(tm)
	if tStr == "" {
		tStr = "00:00"
	}

	if dateStr[1] != "" {
		dd := dateStr[1] + " " + tStr
		date, err := time.Parse("02-Jan-06 15:04", dd)
		if err != nil {
			return t, fmt.Errorf("time parse error %v", err)
		}
		t = date
	}
	if dateStr[2] != "" {
		dd := dateStr[2] + " " + tStr
		date, err := time.Parse("02-01-2006 15:04", dd)
		if err != nil {
			return t, fmt.Errorf("time parse error %v", err)
		}
		t = date
	}
	if dateStr[3] != "" {
		dd := dateStr[3] + " " + tStr
		date, err := time.Parse("2006-01-02 15:04", dd)
		if err != nil {
			return t, fmt.Errorf("time parse error %v", err)
		}
		t = date
	}
	if dateStr[4] != "" {
		dd := dateStr[4] + " " + tStr
		date, err := time.Parse("02/01/06 15:04", dd)
		if err != nil {
			return t, fmt.Errorf("time parse error %v", err)
		}
		t = date
	}
	if dateStr[5] != "" {
		dd := dateStr[5] + " " + tStr
		date, err := time.Parse("02-Jan-2006 15:04", dd)
		if err != nil {
			return t, fmt.Errorf("time parse error %v", err)
		}
		t = date
	}
	if dateStr[6] != "" {
		dd := dateStr[6] + " " + tStr
		date, err := time.Parse("02-01-06 15:04", dd)
		if err != nil {
			return t, fmt.Errorf("time parse error %v", err)
		}
		t = date
	}

	return t, nil
}

func getTransactionType(t string) string {
	_, exist := debitStrMap[t]
	if exist != false {
		return txDebit
	}
	return ""
}

func postMessage(c *gin.Context) {
	var newSMS sms
	var msg message
	var err error

	if err := c.BindJSON(&newSMS); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	smsText := newSMS.M
	bankName := bankRe.FindString(smsText)
	if bankName == "" {
		fmt.Println("Bank name not found in SMS:", smsText)
		c.String(http.StatusOK, "Bank name not found")
		return
	}

	var tMap = map[string]string{
		"amount":   "",
		"txtype":   "",
		"account":  "",
		"date":     "",
		"time":     "",
		"receiver": "",
		"balance":  "",
	}
	for _, p := range patterns {
		matches := p.FindStringSubmatch(smsText)
		if matches != nil {
			names := p.SubexpNames()
			for i, match := range matches {
				if i != 0 {
					tMap[names[i]] = match
				}
			}
			break
		}
	}

	txTypeStr := tMap["txtype"]
	if txTypeStr == "" {
		fmt.Println("Transaction type not found in SMS", smsText)
		c.String(http.StatusOK, "Transaction type not found")
		return
	}
	txType := getTransactionType(txTypeStr)
	if txType == "" {
		fmt.Println("Transaction type parsing error", txTypeStr)
		c.String(http.StatusInternalServerError, "Transaction type parsing error")
		return
	}

	amountStr := tMap["amount"]
	if amountStr == "" {
		fmt.Println("Transaction amount not found in SMS", smsText)
		amountStr = "0"
	}
	amount, err := parseAmount(amountStr)
	if err != nil {
		fmt.Println("Amount parse error:", amountStr)
		c.String(http.StatusInternalServerError, "Amount parsing error")
		return
	}
	txDate, err := parseDateTime(tMap["date"], tMap["time"])
	if err != nil {
		fmt.Println("Date parse error:", tMap["date"], tMap["time"], err)
		txDate = time.Now()
	}
	msg.Amount = amount
	msg.TxType = txType
	msg.Bank = bankName
	msg.Account = tMap["account"]
	msg.DateTime = txDate

	c.IndentedJSON(http.StatusCreated, msg)
}
