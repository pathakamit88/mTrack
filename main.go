package main

import (
	"errors"
	"fmt"
	"github.com/pathakamit88/mTrack/middleware"
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
	Sender   string    `json:"sender"`
	Receiver string    `json:"receiver"`
	Balance  float64   `json:"balance"`
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
	// INR 232.42 spent on ICICI Bank Card XX0000 on 04-Mar-23 at ONE97 COMMUNICA. Avl Lmt: INR 1,23,456.28. To dispute,call 18002662/SMS BLOCK 0000 to 9215676766
	// INR 965.00 spent on ICICI Bank Card XX0000 on 07-Mar-23 at Amazon.in - Gro. Avl Lmt: INR 1,23,456.28. To dispute,call 18002662/SMS BLOCK 0000 to 9215676766
	// INR 634.00 spent on ICICI Bank Card XX0000 on 06-Mar-23 at Amazon. Avl Lmt: INR 1,23,456.28. To dispute,call 18002662/SMS BLOCK 0000 to 9215676766
	// INR 662.74 spent on ICICI Bank Card XX0000 on 04-Mar-23 at UBERINDIASYSTEM. Avl Lmt: INR 1,23,456.54. To dispute,call 18002662/SMS BLOCK 0000 to 9215676766
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
	// Amit, you've received INR 9,000.00 in your Account XXXXXXXX1234. Woohoo! It was sent by 0111 on January 17, 2023. -Federal Bank
	// Amit, you've received INR 2,000.00 in your Account XXXXXXXX0000. Woohoo! It was sent by 0000 on February 6, 2023. -Federal Bank
	regexp.MustCompile(`(?m)you've (?P<txtype>\w+) INR (?P<amount>[\d,.]+) in your Account (?P<account>\w+). Woohoo! It was sent by (?P<sender>[\w]+) on (?P<date>[\w\s,-]+)`),

	// Citibank debit
	// Your Citibank A/c has been debited with INR 194.00 on 07-MAR-2023 at 17:22 and account paytmqr28100505010114n4k18nnfrt@paytm has been credited. UPI Ref no. 30612345219320

	// Citibank ECS debit
	// We confirm ECS debit on your Citi account no. XXXXXX1234 on 06-MAR-23 for an amount of Rs. 12345

	// HDFC food card credit
	// Your HDFC Bank Visa Foodplus Card Card ending with XXXX1234 has been reloaded with INR 1100.00 on 13-JAN-23 03:01 PM. Post Reload Card Bal is INR 1234.67. Not you? Call 18002586161.

	// HDFC CC debit
	// https://regex101.com/r/utoJUX/1
	// You've spent Rs.5 On HDFC Bank CREDIT Card xx0000 At PHONEPE PRIVATE LTD On 2023-01-10:13:39:47 Avl bal: Rs.123456.15 Curr O/s: Rs.14270.85 Not you?Call 18002586161
	// You've spent Rs.2200 On HDFC Bank CREDIT Card xx0000 At ST THERESA EDUCATIONAL On 2023-01-10:11:43:59 Avl bal: Rs.123456 Curr O/s: Rs.14266 Not you?Call 18002586161
	// You've spent Rs.776.33 On HDFC Bank CREDIT Card xx0000 At NHPS CC On 2022-12-30:20:41:47 Avl bal: Rs.123456.24 Curr O/s: Rs.18731.76 Not you?Call 18002586161
	// You've spent Rs.234.82 On HDFC Bank CREDIT Card xx0000 At BBPSBILLPAY On 2022-12-30:13:46:04 Avl bal: Rs.123456.18 Curr O/s: Rs.16433.82 Not you?Call 18002586161
	// You've spent Rs.7394 On HDFC Bank CREDIT Card xx0000 At Decathlon On 2022-12-24:17:18:34 Avl bal: Rs.123456 Curr O/s: Rs.17342 Not you?Call 18002586161
	// You've spent Rs.9947.52 On HDFC Bank CREDIT Card xx0000 At The Paul Bangalore On 2022-12-23:16:13:03 Avl bal: Rs.123456.48 Curr O/s: Rs.9947.52 Not you?Call 18002586161
	// You've spent Rs.920.08 On HDFC Bank CREDIT Card xx0000 At Kids Clinic In Banglor On 2022-12-18:09:57:08 Avl bal: Rs.123456.92 Curr O/s: Rs.1688.08 Not you?Call 18002586161
	regexp.MustCompile(`(?m)You've (?P<txtype>\w+) Rs.(?P<amount>[\d,.]+) On HDFC Bank CREDIT Card (?P<account>\w+) At (?P<receiver>[\w\s.]+) On (?P<date>[\d-]+):(?P<time>[\d:]+) Avl bal: Rs.(?P<balance>[\d,.]+)`),
}

var debitStrMap = map[string]int{
	"Spent":    1,
	"spent":    1,
	"debit":    1,
	"debited":  1,
	"from A/c": 1,
}

var creditStrMap = map[string]int{
	"received": 1,
}

// (\d{2}-\w{3}-\d{2})|(\d{2}-\d{2}-\d{4})|(\d{4}-\d{2}-\d{2})|(\d{2}/\d{2}/\d{2})
var dateRe = regexp.MustCompile(`(?m)(\d{2}-\w{3}-\d{2})|(\d{2}-\d{2}-\d{4})|(\d{4}-\d{2}-\d{2})|(\d{2}/\d{2}/\d{2})|(\d{2}-\w{3}-\d{4})|(\d{2}-\d{2}-\d{2})|(\w+ \d+, \d{4})`)

// \d{2}:\d{2}
var timeRe = regexp.MustCompile(`(?m)\d{2}:\d{2}`)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	r := gin.Default()

	authorized := r.Group("/", middleware.BasicAuthorization())
	authorized.GET("v1/messages", getMessages)
	authorized.POST("v1/messages", postMessage)

	err := r.Run("localhost:8080")
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
		return 0, fmt.Errorf("parseAmount error: %v", err)
	}
	return amount, nil
}

func parseDateTime(dt string, tm string) (time.Time, error) {
	var t time.Time
	var err error
	loc, _ := time.LoadLocation("Asia/Kolkata")

	dateStr := dateRe.FindString(dt)
	if dateStr == "" {
		return t, errors.New("no date string found")
	}
	tStr := timeRe.FindString(tm)
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

func getTransactionType(t string) string {
	var txType string
	_, exist := debitStrMap[t]
	if exist != false {
		txType = txDebit
	}
	_, exist = creditStrMap[t]
	if exist != false {
		txType = txCredit
	}
	return txType
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
		log.Println("Bank name not found in SMS ->"+
			"", smsText)
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
		log.Println("Transaction type not found in SMS ->", smsText)
		c.String(http.StatusOK, "Transaction type not found")
		return
	}
	txType := getTransactionType(txTypeStr)
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
		log.Println("Amount parse error ->", amountStr)
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

	c.IndentedJSON(http.StatusCreated, msg)
}
