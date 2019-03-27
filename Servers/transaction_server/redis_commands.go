package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mediocregopher/radix.v2/redis"
)

func dialRedis() *redis.Client {
	cli, err := redis.Dial("tcp", "redis:6379")
	if err != nil {
		// handle err
	}
	return cli
}

func flushRedis(client *redis.Client) {
	client.Cmd("FLUSHALL")
}

func getBalance(client *redis.Client, username string) float64 {
	getBAL, _ := client.Cmd("HGET", username, "Balance").Float64()
	return getBAL
}

func addBalance(client *redis.Client, username string, amount float64) {
	client.Cmd("HINCRBYFLOAT", username, "Balance", amount)
}

func stockOwned(client *redis.Client, username string, id string) int {
	stocksOwned, _ := client.Cmd("HGET", username, id).Int()
	return stocksOwned
}

func exists(client *redis.Client, username string) bool {
	//client := dialRedis()
	exists, _ := client.Cmd("HGETALL", username).Map()
	if len(exists) == 0 {
		return false
	} else {
		return true
	}
}

func qExists(client *redis.Client, stock string) bool {
	//client := dialRedis()
	exists, _ := client.Cmd("EXISTS", stock).Int()
	fmt.Println("Exists: ", exists, " Stock:", stock)
	if exists == 0 {
		return false
	} else {
		return true
	}
}

func listStack(client *redis.Client, stackName string) []string {
	stack, _ := client.Cmd("LRANGE", stackName, 0, -1).List()
	return stack
}

func listNotEmpty(client *redis.Client, stackName string) bool {
	len, _ := client.Cmd("LLEN", stackName).Int()
	return len != 0
}

func saveTransaction(client *redis.Client, username string, command string, params ...string) {
	// save transaction to HISTORY:username hash set
	timestamp := time.Now().Local()
	timestamp_str := timestamp.String()
	var transaction_string string
	var save bool

	if len(params) == 2 {
		// ADD
		save = true
		amount := params[0]
		newBalance := params[1]

		transaction_string = "[" + string(timestamp_str) + "] " + command + "- Amount: " + amount + ", New Balance: " + newBalance

	} else if len(params) == 5 {
		// COMMIT BUY OR SELL
		save = true
		stock := params[0]
		number := params[1]
		number_int, _ := strconv.Atoi(number)
		if number_int < 1 {
			save = false
		}
		price := params[2]
		totalCost := params[3]
		newBalance := params[4]

		transaction_string = "[" + string(timestamp_str) + "] " + command + "- Stock: " + stock + ", Number: " + number + ", Price: " + price + ", Total Cost: " + totalCost + ", New Balance: " + newBalance

	} else {
		fmt.Println("Error: Wrong number of arguments for saveTransaction()")
		os.Exit(1)
	}

	if save {
		client.Cmd("ZADD", "HISTORY:"+username, timestamp.UnixNano(), transaction_string)
	}
}

func redisADD(client *redis.Client, username string, amount float64) {

	exists := exists(client, username)
	if exists == false {
		client.Cmd("HMSET", username, "User", username, "Balance", amount)
	} else {
		addBalance(client, username, amount)
	}
	newBalance := getBalance(client, username)
	//save to transaction history
	saveTransaction(client, username, "ADD", strconv.FormatFloat(amount, 'f', 2, 64), strconv.FormatFloat(newBalance, 'f', 2, 64))
}

func displayADD(client *redis.Client, username string, amount float64) {
	fmt.Println("-----ADD-----")
	fmt.Println("User: ", username)
	oldBalance := getBalance(client, username)
	fmt.Println("Old Balance: ", oldBalance)
	redisADD(client, username, amount)
	fmt.Println("ADD: ", amount)
	newBalance := getBalance(client, username)
	fmt.Println("New Balance: ", newBalance, "\n")
}

func redisQUOTE(client *redis.Client, transNum int, username string, stock string) {
	stringQ := stock + ":QUOTE"
	ex := qExists(client, stringQ)
	if ex == false {
		go goQuote(client, transNum, username, stock)
	} else {
		stringQ := stock + ":QUOTE"
		currentprice, _ := client.Cmd("GET", stringQ).Float64()
		LogSystemEventCommand(server, transNum, "QUOTE", username, fmt.Sprintf("%f", currentprice), stock, nil)
	}
}

func getQUOTE(client *redis.Client, transNum int, username string, stock string, force bool) float64 {
	if force == true {
		return goQuote(client, transNum, username, stock)
	}
	stringQ := stock + ":QUOTE"
	ex := qExists(client, stringQ)
	if ex == false {
		return goQuote(client, transNum, username, stock)
	}
	currentprice, _ := client.Cmd("GET", stringQ).Float64()
	LogSystemEventCommand(server, transNum, "QUOTE", username, fmt.Sprintf("%f", currentprice), stock, nil)
	return currentprice
}

func goQuote(client *redis.Client, transNum int, username string, stock string) float64 {
	stringQ := stock + ":QUOTE"

	QUOTE_URL := os.Getenv("QUOTE_URL")
	// fmt.Println("quoye url is " + QUOTE_URL)
	conn, _ := net.Dial("tcp", QUOTE_URL)

	conn.Write([]byte((stock + "," + username + "\n")))
	respBuf := make([]byte, 2048)
	_, err := conn.Read(respBuf)
	conn.Close()

	if err != nil {
		fmt.Printf("Error reading body: %s", err.Error())
	}
	respBuf = bytes.Trim(respBuf, "\x00")
	message := bytes.NewBuffer(respBuf).String()
	message = strings.TrimSpace(message)

	fmt.Println(string(message))

	split := strings.Split(message, ",")
	price, error := strconv.ParseFloat(strings.TrimSpace(split[0]), 64)
	if error != nil {
		LogErrorEventCommand(server, transNum, "QUOTE", username, strconv.FormatFloat(price, 'f', 2, 64), stock, nil, "failed in parsing quote stock price into float number")
	}
	quoteTimestamp := strings.TrimSpace(split[3])
	crytpoKey := split[4]

	quoteServerTime := ParseUint(quoteTimestamp, 10, 64)

	LogQuoteServerCommand(server, transNum, strings.TrimSpace(split[0]), stock, username, quoteServerTime, crytpoKey)
	client.Cmd("SET", stringQ, price)
	client.Cmd("EXPIRE", stringQ, 60)

	return price
}

func displayQUOTE(client *redis.Client, transNum int, username string, symbol string) {
	fmt.Println("-----QUOTE-----")
	redisQUOTE(client, transNum, username, symbol)
	stringQ := symbol + ":QUOTE"
	stockPrice, _ := client.Cmd("GET", stringQ).Float64()
	fmt.Println("QUOTE:", stockPrice, "\n")
}

func redisBUY(client *redis.Client, username string, symbol string, totalCost float64, stockSell int) {
	/*
	  check to see buy stack in redis cli
	  LRANGE userBUY:oY01WVirLr 0 -1
	*/
	string3 := "userBUY:" + username
	client.Cmd("LPUSH", string3, symbol)
	client.Cmd("LPUSH", string3, totalCost)
	client.Cmd("LPUSH", string3, stockSell)

}

func displayBUY(client *redis.Client, username string, symbol string, totalCost float64, stockAmount int) {
	fmt.Println("-----BUY-----")
	redisBUY(client, username, symbol, totalCost, stockAmount)
	string3 := "userBUY:" + username
	stack := listStack(client, string3)
	fmt.Println("User:", username, " BUYStack:", stack, "\n")
}

func redisSELL(client *redis.Client, username string, symbol string, totalEarned float64, stockNeeded int) {
	//fmt.Println("newSELL", username, symbol, amount)
	string3 := "userSELL:" + username
	client.Cmd("LPUSH", string3, symbol)
	client.Cmd("LPUSH", string3, totalEarned)
	client.Cmd("LPUSH", string3, stockNeeded)

}

func displaySELL(client *redis.Client, username string, symbol string, totalEarned float64, stockNeeded int) {
	fmt.Println("-----SELL-----")
	redisSELL(client, username, symbol, totalEarned, stockNeeded)
	string3 := "userSELL:" + username
	stack := listStack(client, string3)
	fmt.Println("User: ", username, "SELLStack: ", stack, "\n")
}

func redisCOMMIT_BUY(client *redis.Client, username string, transNum int) string {

	/*
	  1. LPOP - stock symbol
	  2. LPOP - amount
	  3. check user balance
	  4. calculate num. stocks to buy
	  5. decrease balance
	  6. increase stocks
	*/

	string3 := "userBUY:" + username
	stockBuy, _ := client.Cmd("LPOP", string3).Int()
	totalCost, _ := client.Cmd("LPOP", string3).Float64()
	stock, _ := client.Cmd("LPOP", string3).Str()

	getBAL := getBalance(client, username)

	if getBAL < totalCost {
		LogErrorEventCommand(server, transNum, "COMMIT_BUY", username, strconv.FormatFloat(totalCost, 'f', 2, 64), nil, nil, "user "+username+" doesn not have enough balance to buy stock "+stock)
		return "balace is not enough to commit buy"
	}

	/* 5 */
	addBalance(client, username, -totalCost)
	LogAccountTransactionCommand(server, transNum, "COMMIT_BUY", username, strconv.FormatFloat(totalCost, 'f', 2, 64))

	/* 6 */
	id := stock + ":OWNED"

	if stockBuy > 0 {
		client.Cmd("HINCRBY", username, id, stockBuy)
		// add stock to OWNED:[username] for ease of access in display summary
		client.Cmd("HINCRBY", "OWNED:"+username, id, stockBuy)
	}

	getBAL2 := getBalance(client, username)
	stockUnitPrice := totalCost / float64(stockBuy)
	saveTransaction(client, username, "COMMIT_BUY", stock, string(stockBuy), strconv.FormatFloat(stockUnitPrice, 'f', 2, 64), strconv.FormatFloat(totalCost, 'f', 2, 64), strconv.FormatFloat(getBAL2, 'f', 2, 64))
	return ""
}

func displayCOMMIT_BUY(client *redis.Client, username string, transNum int) string {
	//string3 := "userBUY:" + username
	//fmt.Println("SYMBOL:", stock, "AMOUNT:", amount)
	fmt.Println("-----COMMIT_BUY-----")
	fmt.Println("User: ", username)
	/* 3 */
	getBAL := getBalance(client, username)
	fmt.Println("Old Balance:", getBAL)

	message := redisCOMMIT_BUY(client, username, transNum)

	getBAL2 := getBalance(client, username)
	fmt.Println("New Balance:", getBAL2, "\n")

	string3 := "userBUY:" + username
	stack := listStack(client, string3)
	fmt.Println("User:", username, " BUYStack:", stack, "\n")
	return message
}

func redisCOMMIT_SELL(client *redis.Client, username string, transNum int) string {

	/*
	  1. LPOP - stock symbol
	  2. LPOP - amount
	  4. calculate num. stocks to buy
	  5. increase balance
	  6. decrease stocks
	*/

	/* 1, 2 */
	string3 := "userSELL:" + username
	stockNeeded, _ := client.Cmd("LPOP", string3).Int()
	totalEarned, _ := client.Cmd("LPOP", string3).Float64()
	stock, _ := client.Cmd("LPOP", string3).Str()

	/* 4 */
	id := stock + ":OWNED"
	stocksOwned := stockOwned(client, username, id)

	/* 5 */

	if stocksOwned < stockNeeded {
		LogErrorEventCommand(server, transNum, "COMMIT_SELL", username, strconv.FormatFloat(totalEarned, 'f', 2, 64), nil, nil, "user "+username+" doesn't have enough stock "+stock+" to sell ")
		return "stock owned is not enough to sell"
	}

	addBalance(client, username, totalEarned)
	LogAccountTransactionCommand(server, transNum, "COMMIT_SELL", username, strconv.FormatFloat(totalEarned, 'f', 2, 64))

	getBAL3 := getBalance(client, username)

	/* 6 */
	if stockNeeded > 0 {
		client.Cmd("HINCRBY", username, id, -stockNeeded)
		saveTransaction(client, username, "COMMIT_SELL", stock, string(stockNeeded), strconv.FormatFloat(totalEarned/float64(stockNeeded), 'f', 2, 64), strconv.FormatFloat(totalEarned, 'f', 2, 64), strconv.FormatFloat(getBAL3, 'f', 2, 64))
	}

	return ""

}

func displayCOMMIT_SELL(client *redis.Client, username string, transNum int) string {
	fmt.Println("-----COMMIT_SELL-----")
	fmt.Println("User: ", username)
	getBAL := getBalance(client, username)
	fmt.Println("Old Balance:", getBAL)

	message := redisCOMMIT_SELL(client, username, transNum)

	getBAL2 := getBalance(client, username)
	fmt.Println("New Balance:", getBAL2, "\n")

	string3 := "userSELL:" + username
	stack := listStack(client, string3)
	fmt.Println("User:", username, " SELLStack:", stack, "\n")
	return message
}

func redisCANCEL_BUY(client *redis.Client, username string) {

	/* Pop off 2 items from buy stack: stock name, and $ amount */
	string3 := "userBUY:" + username
	client.Cmd("LPOP", string3).Int()
	client.Cmd("LPOP", string3).Float64()
	client.Cmd("LPOP", string3).Str()
	//fmt.Println("Stock:", stock, "Amount:", amount)

}
func displayCANCEL_BUY(client *redis.Client, username string) {
	fmt.Println("-----CANCEL_BUY-----")
	redisCANCEL_BUY(client, username)
	string3 := "userBUY:" + username
	stack := listStack(client, string3)
	fmt.Println("User:", username, " BUYStack:", stack, "\n")
}

func redisCANCEL_SELL(client *redis.Client, username string) {

	/* Pop off 2 items from buy stack: stock name, and $ amount */
	string3 := "userSELL:" + username
	client.Cmd("LPOP", string3).Int()
	client.Cmd("LPOP", string3).Float64()
	client.Cmd("LPOP", string3).Str()
	//fmt.Println("Stock:", stock, "Amount:", amount)

}

func displayCANCEL_SELL(client *redis.Client, username string) {
	fmt.Println("-----CANCEL_SELL-----")
	redisCANCEL_SELL(client, username)
	string3 := "userSELL:" + username
	stack := listStack(client, string3)
	fmt.Println("User:", username, " SELLStack:", stack, "\n")
}

func redisSET_BUY_AMOUNT(client *redis.Client, username string, symbol string, amount float64) {

	string3 := symbol + ":BUY:" + username
	client.Cmd("LPUSH", string3, amount)

	/*
	  push amount then trigger price in SET_BUY_TRIGGER
	  these two operate in pairs
	*/

	/*
	  decrease balance put in reserve
	*/

	client.Cmd("HINCRBYFLOAT", username, "Balance", -amount)

}

func displaySET_BUY_AMOUNT(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_BUY_AMOUNT-----")
	redisSET_BUY_AMOUNT(client, username, symbol, amount)
	fmt.Println("Username: ", username)

	string3 := symbol + ":BUY:" + username
	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SETBUYAMOUNTstack for ", symbol, ":", stack)

	getBAL2 := getBalance(client, username)
	fmt.Println("NEWBalance:", getBAL2, "\n")
}

func redisSET_BUY_TRIGGER(client *redis.Client, username string, symbol string, amount float64) {

	string3 := symbol + ":BUYTRIG"
	client.Cmd("HSET", username, string3, amount)
	client.Cmd("HSET", "BUYTRIGGERS:"+username, symbol, amount)
}

func displaySET_BUY_TRIGGER(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_BUY_TRIGGER-----")
	fmt.Println("Username: ", username)

	redisSET_BUY_TRIGGER(client, username, symbol, amount)

	string3 := "BUYTRIGGERS:" + username
	triggers, _ := client.Cmd("HGETALL", string3).List()
	fmt.Println("BUYTRIGGERS: ", triggers, "\n")
}

func redisCANCEL_SET_BUY(client *redis.Client, username string, symbol string) {

	/* get length of stack */
	string3 := symbol + ":BUY:" + username

	stackLength, _ := client.Cmd("LLEN", string3).Int()
	//fmt.Println("Stack length:", stackLength)

	for i := 0; i < stackLength; i++ {
		refund, _ := client.Cmd("LPOP", string3).Float64()
		addBalance(client, username, refund)
		//getBAL2 := getBalance(client, username)
		//fmt.Println("Refund:", refund)
		//fmt.Println("New Balance:", getBAL2)
	}

	string4 := symbol + ":BUYTRIG"
	client.Cmd("HSET", username, string4, 0.00)
	string5 := "BUYTRIGGERS:" + username
	client.Cmd("HSET", string5, symbol, 0.00)
}

func displayCANCEL_SET_BUY(client *redis.Client, username string, symbol string) {
	fmt.Println("-----CANCEL_SET_BUY-----")
	fmt.Println("Username: ", username)
	redisCANCEL_SET_BUY(client, username, symbol)

	string3 := symbol + ":BUY:" + username
	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SETBUYAMOUNTstack for ", symbol, ":", stack)

	string4 := "BUYTRIGGERS:" + username
	triggers, _ := client.Cmd("HGETALL", string4).List()
	fmt.Println("BUYTRIGGERS: ", triggers)

	getBAL2 := getBalance(client, username)
	fmt.Println("NEWBalance:", getBAL2, "\n")
}

func redisSET_SELL_AMOUNT(client *redis.Client, username string, symbol string, amount float64) {

	string3 := symbol + ":SELL:" + username
	client.Cmd("LPUSH", string3, amount)

	/*
	  push amount then trigger price in SET_BUY_TRIGGER
	  these two operate in pairs
	*/
	//stack := listStack(client, string3)
	//fmt.Println("SETSELLTRIGGERStack: ", stack)
}

func displaySET_SELL_AMOUNT(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_SELL_AMOUNT-----")
	redisSET_SELL_AMOUNT(client, username, symbol, amount)

	fmt.Println("Username: ", username)
	string3 := symbol + ":SELL:" + username
	stack := listStack(client, string3)
	fmt.Println("SETSELLAMOUNTStack for ", symbol, ": ", stack, "\n")
}

func redisSET_SELL_TRIGGER(client *redis.Client, username string, symbol string, amount float64) {

	string3 := symbol + ":SELLTRIG"
	client.Cmd("HSET", username, string3, amount)

	client.Cmd("HSET", "SELLTRIGGERS:"+username, symbol, amount)
}

func displaySET_SELL_TRIGGER(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_SELL_TRIGGER-----")
	fmt.Println("Username: ", username)
	redisSET_SELL_TRIGGER(client, username, symbol, amount)

	triggers, _ := client.Cmd("HGETALL", "SELLTRIGGERS:"+username).List()
	fmt.Println("SELLTRIGGERS: ", triggers, "\n")
}

func redisCANCEL_SET_SELL(client *redis.Client, username string, symbol string) {
	/* get length of stack */
	string3 := symbol + ":SELL:" + username
	stackLength, _ := client.Cmd("LLEN", string3).Int()
	//fmt.Println("Stack length:", stackLength)

	for i := 0; i < stackLength; i++ {
		client.Cmd("LPOP", string3).Float64()
	}
	string4 := symbol + ":SELLTRIG"
	client.Cmd("HSET", username, string4, 0.00)
}

func displayCANCEL_SET_SELL(client *redis.Client, username string, symbol string) {
	fmt.Println("-----CANCEL_SET_SELL-----")
	fmt.Println("Username: ", username)
	redisCANCEL_SET_SELL(client, username, symbol)

	string3 := symbol + ":SELL:" + username
	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SETSELLAMOUNTstack for ", symbol, ":", stack)

	string4 := "SELLTRIGGERS:" + username
	triggers, _ := client.Cmd("HGETALL", string4).List()
	fmt.Println("SELLTRIGGERS: ", triggers)

	getBAL2 := getBalance(client, username)
	fmt.Println("NEWBalance:", getBAL2, "\n")
}

func redisDISPLAY_SUMMARY(client *redis.Client, username string) {
	fmt.Println("-----DISPLAY_SUMMARY-----")

	fmt.Println("Username: ", username)
	fmt.Println("Balance: ", getBalance(client, username))
	stocks_owned, _ := client.Cmd("HGETALL", "OWNED:"+username).Map()
	fmt.Println("Stocks owned:")
	for key, val := range stocks_owned {
		fmt.Println(strings.Split("    "+key, ":")[0] + ": " + val)
	}
	fmt.Println("Buy triggers: ")
	buy_triggers, _ := client.Cmd("HGETALL", "BUYTRIGGERS:"+username).Map()
	sell_triggers, _ := client.Cmd("HGETALL", "SELLTRIGGERS:"+username).Map()
	for key, val := range buy_triggers {
		fmt.Println(strings.Split("    "+key, ":")[0] + ": " + val)
	}
	fmt.Println("Sell triggers: ")
	for key, val := range sell_triggers {
		fmt.Println(strings.Split("    "+key, ":")[0] + ": " + val)
	}
	fmt.Println("Transaction history: ")
	client.Cmd("SORT", "HISTORY:"+username)
	history, _ := client.Cmd("ZRANGE", "HISTORY:"+username, 0, -1).List()
	for _, val := range history {
		fmt.Println("    " + val)
	}
}
