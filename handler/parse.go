package handler

import (
	"errors"
	"fmt"
	"github.com/pathakamit88/txsms"
	"strconv"
	"strings"
	"time"
)

const txDebit = "Debit"
const txCredit = "Credit"

func parseAmount(m string) (float64, error) {
	amountStr := strings.Replace(m, ",", "", -1)
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parseAmount error: %v", err)
	}
	return amount, nil
}

func parseDateTime(dt string, tm string) (time.Time, error) {
	var t time.Time
	var err error
	loc, _ := time.LoadLocation("Asia/Kolkata")

	dateStr := txsms.DateRe.FindString(dt)
	if dateStr == "" {
		return t, errors.New("no date string found")
	}
	tStr := txsms.TimeRe.FindString(tm)
	if tStr == "" {
		tStr = time.Now().In(loc).Format("15:04")
	}

	dateStr = dateStr + " " + tStr
	var dateLayout = []string{
		"02-Jan-06 15:04",
		"02-01-2006 15:04",
		"2006-01-02 15:04",
		"02/01/06 15:04",
		"02-Jan-2006 15:04",
		"02-01-06 15:04",
		"January 2, 2006 15:04",
	}
	for _, l := range dateLayout {
		t, err = time.ParseInLocation(l, dateStr, loc)
		if err == nil {
			break
		}

	}
	if err != nil {
		return time.Now().In(loc), err
	}
	return t, nil
}

func parseTransactionType(t string) string {
	var txType string
	_, exist := txsms.DebitStrMap[t]
	if exist != false {
		txType = txDebit
	}
	_, exist = txsms.CreditStrMap[t]
	if exist != false {
		txType = txCredit
	}
	return txType
}
