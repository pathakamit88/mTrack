package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
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

var debitRe = regexp.MustCompile(
	`(?m)(spent|debited|from A/c)`)

// ([\d\,]+\.?\d{2}) (spent|debited|from A/c|On HDFC)
var amountRe = regexp.MustCompile(
	`(?m)([\d,]+\.?\d{2}) (spent|debited|from A/c|On HDFC)`)

// (\d{2}-\w{3}-\d{2})|(\d{2}-\d{2}-\d{4})|(\d{4}-\d{2}-\d{2})
var dateRe = regexp.MustCompile(
	`(?m)(\d{2}-\w{3}-\d{2})|(\d{2}-\d{2}-\d{4})|(\d{4}-\d{2}-\d{2})`)

// \d{2}:\d{2}
var timeRe = regexp.MustCompile(`(?m)\d{2}:\d{2}`)

// ICICI|Federal|SBI
var bankRe = regexp.MustCompile(
	`(?m)ICICI|Federal|SBI|HDFC`)

// (A/c|a/c|Card|card) ([xX]+)?\d+
var accountRe = regexp.MustCompile(`(?m)(A/c|a/c|Card|card) ([xX]+)?\d+`)

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
	amountString := amountRe.FindStringSubmatch(m)
	if amountString == nil {
		return 0, fmt.Errorf("parseAmount no amount")
	}
	amount, err := strconv.ParseFloat(amountString[1], 64)
	if err != nil {
		log.Println(err)
		return 0, fmt.Errorf("parseAmount error: %v", err)
	}
	return amount, nil
}

func parseDate(m string) (time.Time, error) {
	var t time.Time

	tStr := timeRe.FindString(m)
	if tStr == "" {
		tStr = "00:00"
	}

	dateStr := dateRe.FindStringSubmatch(m)
	if dateStr == nil {
		return t, errors.New("no date string found")
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
	return t, nil
}

func parseAccount(m string) (string, error) {
	accountStr := accountRe.FindString(m)
	if accountStr == "" {
		return "", errors.New("account not found")
	}
	return accountStr, nil
}

func parseDebitSMS(bank string, m string) (message, error) {
	var msg message

	amount, err := parseAmount(m)
	if err != nil {
		log.Println(err)
		return msg, fmt.Errorf("parseAmount error: %v", err)
	}

	date, err := parseDate(m)
	if err != nil {
		log.Println(err)
		return msg, fmt.Errorf("parseDate error: %v", err)
	}

	account, _ := parseAccount(m)

	msg.TxType = txDebit
	msg.Bank = bank
	msg.Account = account
	msg.Amount = amount
	msg.DateTime = date
	return msg, nil
}

func postMessage(c *gin.Context) {
	var newSMS sms
	var msg message
	var err error

	if err := c.BindJSON(&newSMS); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	bankName := bankRe.FindString(newSMS.M)
	if bankName == "" {
		// Return 200 as this is not a banking sms.
		return
	}

	isDebit := debitRe.FindStringSubmatch(newSMS.M)
	if isDebit != nil {
		msg, err = parseDebitSMS(bankName, newSMS.M)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		messages = append(messages, msg)
	}

	c.IndentedJSON(http.StatusCreated, msg)
}
