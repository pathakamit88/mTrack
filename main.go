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

// ICICI|Federal|SBI
var bankRe = regexp.MustCompile(`(?m)ICICI|Federal|SBI|HDFC|Citi|Axis`)

var patterns = []*regexp.Regexp{
	// Axis Bank debit
	// Spent Card no. XX0000 INR 1579 13-12-22 19:57:25 ABCDEFG IND Avl Lmt INR 123456.05 SMS BLOCK 1696 to 918691000002, if not you - Axis Bank
	regexp.MustCompile(`(?m)(?P<txtype>\w+) Card no. (?P<account>\w+) INR (?P<amount>[\d,.]+) (?P<date>[\d-]+) (?P<time>[\d:]+) (?P<receiver>[\w\s]+) Avl Lmt INR (?P<balance>[\d,.]+)`),

	// ICICI debit
	// https://regex101.com/r/8QNj0Q/1
	// INR 232.42 spent on ICICI Bank Card XX0000 on 04-Mar-23 at ONE97 COMMUNICA. Avl Lmt: INR 1,23,456.28. To dispute,call 18002662/SMS BLOCK 0000 to 9215676766
	// INR 965.00 spent on ICICI Bank Card XX0000 on 07-Mar-23 at Amazon.in - Gro. Avl Lmt: INR 1,23,456.28. To dispute,call 18002662/SMS BLOCK 0000 to 9215676766
	// INR 634.00 spent on ICICI Bank Card XX3002 on 06-Mar-23 at Amazon. Avl Lmt: INR 2,48,113.28. To dispute,call 18002662/SMS BLOCK 3002 to 9215676766
	// INR 662.74 spent on ICICI Bank Card XX3002 on 04-Mar-23 at UBERINDIASYSTEM. Avl Lmt: INR 2,38,513.54. To dispute,call 18002662/SMS BLOCK 3002 to 9215676766
	regexp.MustCompile(`(?m)INR (?P<amount>[\d,.]+) (?P<txtype>\w+) on ICICI Bank Card (?P<account>\w+) on (?P<date>[\w-]+) at (?P<receiver>[\w\s\-.]+). Avl Lmt: INR (?P<balance>[\d,]+.?\d{2}?)`),

	// Federal bank debit
	// https://regex101.com/r/yUjsna/1
	//Rs 706.82 debited from your A/c using UPI on 07-03-2023 19:57:24 to VPA godaddy.cca@hdfcbank - (UPI Ref No 300000882989)-Federal Bank
	//Rs 70.00 debited from your A/c using UPI on 06-03-2023 10:33:05 to VPA 77579656006119@cnrb - (UPI Ref No 306510599225)-Federal Bank
	//Rs 1000.00 debited from your A/c using UPI on 20-02-2023 13:03:57 to VPA npstrust.billdesk@hdfcbank - (UPI Ref No 305113954210)-Federal Bank
	//Rs 65.00 debited from your A/c using UPI on 18-02-2023 15:12:13 to VPA budgetmart1@fbl - (UPI Ref No 304915283287)-Federal Bank
	//Rs 191.80 debited from your A/c using UPI on 09-02-2023 16:34:47 and VPA paytm-irctcapp@paytm credited (UPI Ref No 304045190301)-Federal Bank
	//Rs 260.00 debited from your A/c using UPI on 18-01-2023 17:11:41 and VPA q771711303@ybl credited (UPI Ref No 301811966717)-Federal Bank
	// Rs 40.00 debited from your A/c using UPI on 16-01-2023 10:05:01 and VPA q32471588@ybl credited (UPI Ref No 301604954573)-Federal Bank
	regexp.MustCompile(`(?m)Rs (?P<amount>[\d,.]+) (?P<txtype>\w+) from your A/c using UPI on (?P<date>[\w-]+) (?P<time>[\d:]+) (to|and) VPA (?P<receiver>[\w-.@]+)`),

	// Federal bank credit
	// Amit, you've received INR 9,000.00 in your Account XXXXXXXX9331. Woohoo! It was sent by 0111 on January 17, 2023. -Federal Bank
	// Amit, you've received INR 2,000.00 in your Account XXXXXXXX9331. Woohoo! It was sent by 0000 on February 6, 2023. -Federal Bank

	// Citibank debit
	// Your Citibank A/c has been debited with INR 194.00 on 07-MAR-2023 at 17:22 and account paytmqr28100505010114n4k18nnfrt@paytm has been credited. UPI Ref no. 306655219320

	// Citibank ECS debit
	// We confirm ECS debit on your Citi account no. XXXXXX9228 on 06-MAR-23 for an amount of Rs. 40817

	// HDFC food card credit
	// Your HDFC Bank Visa Foodplus Card Card ending with XXXX1879 has been reloaded with INR 1100.00 on 13-JAN-23 03:01 PM. Post Reload Card Bal is INR 1248.67. Not you? Call 18002586161.

	// HDFC CC debit
	// https://regex101.com/r/utoJUX/1
	// You've spent Rs.5 On HDFC Bank CREDIT Card xx6530 At PHONEPE PRIVATE LTD On 2023-01-10:13:39:47 Avl bal: Rs.684729.15 Curr O/s: Rs.14270.85 Not you?Call 18002586161
	// You've spent Rs.2200 On HDFC Bank CREDIT Card xx6530 At ST THERESA EDUCATIONAL On 2023-01-10:11:43:59 Avl bal: Rs.684734 Curr O/s: Rs.14266 Not you?Call 18002586161
	// You've spent Rs.776.33 On HDFC Bank CREDIT Card xx6530 At NHPS CC On 2022-12-30:20:41:47 Avl bal: Rs.680268.24 Curr O/s: Rs.18731.76 Not you?Call 18002586161
	// You've spent Rs.234.82 On HDFC Bank CREDIT Card xx6530 At BBPSBILLPAY On 2022-12-30:13:46:04 Avl bal: Rs.682566.18 Curr O/s: Rs.16433.82 Not you?Call 18002586161
	// You've spent Rs.7394 On HDFC Bank CREDIT Card xx6530 At Decathlon On 2022-12-24:17:18:34 Avl bal: Rs.681658 Curr O/s: Rs.17342 Not you?Call 18002586161
	// You've spent Rs.9947.52 On HDFC Bank CREDIT Card xx6530 At The Paul Bangalore On 2022-12-23:16:13:03 Avl bal: Rs.689052.48 Curr O/s: Rs.9947.52 Not you?Call 18002586161
	// You've spent Rs.920.08 On HDFC Bank CREDIT Card xx6530 At Kids Clinic In Banglor On 2022-12-18:09:57:08 Avl bal: Rs.697311.92 Curr O/s: Rs.1688.08 Not you?Call 18002586161
	regexp.MustCompile(`(?m)You've (?P<txtype>\w+) Rs.(?P<amount>[\d,.]+) On HDFC Bank CREDIT Card (?P<account>\w+) At (?P<receiver>[\w\s.]+) On (?P<date>[\d-]+):(?P<time>[\d:]+) Avl bal: Rs.(?P<balance>[\d,.]+)`),
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
