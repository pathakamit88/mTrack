package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/pathakamit88/txsms"
	"log"
	"net/http"
	"time"
)

type sms struct {
	M string `json:"m"`
}

type message struct {
	TxType   string    `json:"type"`
	Bank     string    `json:"bank"`
	Account  string    `json:"account"`
	Amount   float64   `json:"amount"`
	Sender   string    `json:"sender"`
	Receiver string    `json:"receiver"`
	Balance  float64   `json:"balance"`
	DateTime time.Time `json:"time"`
}

var messages []message

func GetMessages(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, messages)
}

func PostMessage(c *gin.Context) {
	var newSMS sms
	var msg message
	var err error

	if err := c.BindJSON(&newSMS); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	smsText := newSMS.M

	bankName := txsms.BankRe.FindString(smsText)
	if bankName == "" {
		c.Status(http.StatusOK)
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
	for _, p := range txsms.SmsPatternsRe {
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
		log.Println("Transaction type not found in SMS", smsText)
		c.String(http.StatusOK, "Transaction type not found")
		return
	}
	txType := parseTransactionType(txTypeStr)
	if txType == "" {
		log.Println("Transaction type parsing error ->", txTypeStr)
		c.String(http.StatusInternalServerError, "Transaction type parsing error")
		return
	}

	amountStr := tMap["amount"]
	if amountStr == "" {
		log.Println("Transaction amount not found in SMS ->", smsText)
		amountStr = "0"
	}
	amount, err := parseAmount(amountStr)
	if err != nil {
		log.Println("Amount parse error ->", amountStr, smsText)
		c.String(http.StatusInternalServerError, "Amount parsing error")
		return
	}

	txDate, err := parseDateTime(tMap["date"], tMap["time"])
	if err != nil {
		log.Println("Date parse error ->", tMap["date"], tMap["time"], err)
	}

	balanceStr := tMap["balance"]
	if balanceStr == "" {
		balanceStr = "0"
	}
	balance, err := parseAmount(balanceStr)
	if err != nil {
		log.Println("Balance amount parse error ->", balanceStr)
		c.String(http.StatusInternalServerError, "Balance amount parsing error")
		return
	}

	msg.Amount = amount
	msg.TxType = txType
	msg.Bank = bankName
	msg.Account = tMap["account"]
	msg.DateTime = txDate
	msg.Receiver = tMap["receiver"]
	msg.Balance = balance

	messages = append(messages, msg)
	c.IndentedJSON(http.StatusCreated, msg)
}
