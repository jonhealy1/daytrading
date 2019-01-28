package main

import (
	"fmt"
	"math"
	"reflect"

	"github.com/mediocregopher/radix.v2/redis"
)

func dialRedis() *redis.Client {
	cli, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		// handle err
	}
	return cli
}

func getBalance(client *redis.Client, username string) float64 {
	getBAL, _ := client.Cmd("HGET", username, "Balance").Float64()
	return getBAL
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

func redisADD(client *redis.Client, username string, amount float64) {
	fmt.Println("newADD", username, amount)
	exists := exists(client, username)
	if exists == false {
		client.Cmd("HMSET", username, "User", username, "Balance", amount)
	} else {
		client.Cmd("HINCRBYFLOAT", username, "Balance", amount)
	}
	/* Display - get new balance (HGET) */
	fmt.Print("ADD:   ", amount)
	x, _ := client.Cmd("HGET", username, "Balance").Float64()
	fmt.Println(" Balance: ", x)
}
func redisQUOTE(client *redis.Client, username string, symbol string) {
	fmt.Println("-----QUOTE-----")
	stockPrice, _ := client.Cmd("HGET", username, "QUOTE").Float64()
	fmt.Println("QUOTE:", stockPrice)

	/* HINCRBYFLOAT: change a float value. Quote costs a User
	$0.50 */
	client.Cmd("HINCRBYFLOAT", username, "Balance", -0.50)
	//price := stockPrices[symbol]
	//fmt.Println("QUOTE2:", price)
}

func redisBUY(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("newBUY", username, symbol, amount)

	/*
		check to see buy stack in redis cli
		LRANGE userBUY:oY01WVirLr 0 -1
	*/

	string3 := "userBUY:" + username
	client.Cmd("LPUSH", string3, amount)
	client.Cmd("LPUSH", string3, symbol)

	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("BUYStack: ", stack)
}

func redisSELL(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("newSELL", username, symbol, amount)
	string3 := "userSELL:" + username
	client.Cmd("LPUSH", string3, amount)
	client.Cmd("LPUSH", string3, symbol)
	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SELLStack: ", stack)
}
func redisCOMMIT_BUY(client *redis.Client, username string) {
	fmt.Println("-----COMMIT_BUY-----")

	/*
		1. LPOP - stock symbol
		2. LPOP - amount
		3. check user balance
		4. calculate num. stocks to buy
		5. decrease balance
		6. increase stocks
	*/

	/* 1, 2 */
	string3 := "userBUY:" + username
	stock, _ := client.Cmd("LPOP", string3).Str()
	amount, _ := client.Cmd("LPOP", string3).Float64()
	fmt.Println("SYMBOL:", stock, "AMOUNT:", amount)

	/* 3 */
	getBAL := getBalance(client, username)
	fmt.Println("Balance:", getBAL)

	/* 4 */
	//stockPrice := stockPrices[stock]
	stockPrice, _ := client.Cmd("HGET", username, "QUOTE").Float64()
	fmt.Println("QUOTE:", stockPrice)
	stock2BUY := int(math.Floor(amount / stockPrice))
	totalCOST := stockPrice * float64(stock2BUY)
	fmt.Println("Price:", stockPrice, "BUYAmount:", stock2BUY)
	fmt.Println("TotalCost:", totalCOST)

	/* 5 */
	client.Cmd("HINCRBYFLOAT", username, "Balance", -totalCOST)
	getBAL2 := getBalance(client, username)
	fmt.Println("NEWBalance:", getBAL2)

	/* 6 */
	fmt.Println(reflect.TypeOf(stockPrice))
	stringX := stock + ":OWNED"

	//stockXX, _ := client.Cmd("HGET", username, "QUOTE").Float64()

	if stock2BUY > 0 {
		client.Cmd("HINCRBYFLOAT", username, stringX, stock2BUY)

	}
	stockOWNS, _ := client.Cmd("HGET", username, stringX).Float64()
	fmt.Println("Stock: ", stock, "TOTAL: ", stockOWNS)

}

func redisCOMMIT_SELL(client *redis.Client, username string) {
	fmt.Println("-----COMMIT_SELL-----")

	/*
		1. LPOP - stock symbol
		2. LPOP - amount
		4. calculate num. stocks to buy
		5. increase balance
		6. decrease stocks
	*/

	/* 1, 2 */
	string3 := "userSELL:" + username
	stock, _ := client.Cmd("LPOP", string3).Str()
	amount, _ := client.Cmd("LPOP", string3).Float64()
	fmt.Println("SYMBOL:", stock, "AMOUNT:", amount)

	/* 4 */
	stockPrice, _ := client.Cmd("HGET", username, "QUOTE").Float64()
	fmt.Println("QUOTE:", stockPrice)
	stock2SELL := int(math.Floor(amount / stockPrice))
	totalCOST := stockPrice * float64(stock2SELL)
	fmt.Println("Price:", stockPrice, "SELLAmount:", stock2SELL)
	fmt.Println("TotalCost:", totalCOST)

	/* 5 */
	client.Cmd("HINCRBYFLOAT", username, "Balance", totalCOST)
	getBAL3 := getBalance(client, username)
	fmt.Println("NEWBalance:", getBAL3)

	/* 6 */
	//fmt.Println(reflect.TypeOf(stockPrice))
	stringX := stock + ":OWNED"
	fmt.Println(stringX)

	//stockXX, _ := client.Cmd("HGET", username, "QUOTE").Float64()

	if stock2SELL > 0 {
		client.Cmd("HINCRBYFLOAT", username, stringX, -stock2SELL)

	}
	stockOWNS, _ := client.Cmd("HGET", username, stringX).Float64()
	fmt.Println("Stock: ", stock, "TOTAL: ", stockOWNS)

}

func redisCANCEL_BUY(client *redis.Client, username string) {

	fmt.Println("-----CANCEL_BUY-----")
	/* Pop off 2 items from buy stack: stock name, and $ amount */
	string3 := "userBUY:" + username
	stock, _ := client.Cmd("LPOP", string3).Str()
	amount, _ := client.Cmd("LPOP", string3).Float64()
	fmt.Println("Stock:", stock, "Amount:", amount)

}

func redisCANCEL_SELL(client *redis.Client, username string) {

	fmt.Println("-----CANCEL_SELL-----")
	/* Pop off 2 items from buy stack: stock name, and $ amount */
	string3 := "userSELL:" + username
	stock, _ := client.Cmd("LPOP", string3).Str()
	amount, _ := client.Cmd("LPOP", string3).Float64()
	fmt.Println("Stock:", stock, "Amount:", amount)

}

func redisSET_BUY_AMOUNT(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_BUY_AMOUNT-----")
	string3 := symbol + ":BUY:" + username
	client.Cmd("LPUSH", string3, amount)

	/*
		push amount then trigger price in SET_BUY_TRIGGER
		these two operate in pairs
	*/

	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SETBUYTRIGGERStack: ", stack)

	/*
		decrease balance put in reserve
	*/

	client.Cmd("HINCRBYFLOAT", username, "Balance", -amount)
	getBAL2 := getBalance(client, username)
	fmt.Println("NEWBalance:", getBAL2)
}
func redisSET_BUY_TRIGGER(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_BUY_TRIGGER-----")
	string3 := symbol + ":BUY:" + username
	client.Cmd("LPUSH", string3, amount)
	//client.Cmd("LPUSH", string3, symbol)

	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SETBUYTRIGGERStack: ", stack)
}
func redisCANCEL_SET_BUY(client *redis.Client, username string, symbol string) {
	fmt.Println("-----CANCEL_SET_BUY-----")
	/* get length of stack */
	string3 := symbol + ":BUY:" + username
	stackLength, _ := client.Cmd("LLEN", string3).Int()
	fmt.Println("Stack length:", stackLength)
	/* if even number of items on stack
	refund second pop
	*/
	if stackLength%2 == 0 {
		client.Cmd("LPOP", string3)
		refund, _ := client.Cmd("LPOP", string3).Float64()
		client.Cmd("HINCRBYFLOAT", username, "Balance", refund)
		getBAL2 := getBalance(client, username)
		fmt.Println("NEWBalance(refund):", getBAL2)
		/* else if odd # refund first pop */
	} else if stackLength%2 == 1 {
		//client.Cmd("LPOP", string3)
		refund, _ := client.Cmd("LPOP", string3).Float64()
		client.Cmd("HINCRBYFLOAT", username, "Balance", refund)
		getBAL2 := getBalance(client, username)
		fmt.Println("NEWBalance(refund):", getBAL2)
	}

}

func redisSET_SELL_AMOUNT(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_SELL_AMOUNT-----")
	string3 := symbol + ":SELL:" + username
	client.Cmd("LPUSH", string3, amount)

	/*
		push amount then trigger price in SET_BUY_TRIGGER
		these two operate in pairs
	*/

	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SETSELLTRIGGERStack: ", stack)

	//sellStr := symbol + ":OWNED"
	//client.Cmd("HINCRBYFLOAT", username, sellStr, -amount)
	//getBAL2 := getBalance(client, username)
	//fmt.Println("NEWBalance:", getBAL2)
}
func redisSET_SELL_TRIGGER(client *redis.Client, username string, symbol string, amount float64) {
	fmt.Println("-----SET_SELL_TRIGGER-----")
	string3 := symbol + ":SELL:" + username

	//client.Cmd("LPUSH", string3, symbol)

	stack, _ := client.Cmd("LRANGE", string3, 0, -1).List()
	fmt.Println("SETSELLTRIGGERStack: ", stack)

	/* must set aside stocks on reserve */
	/* TODO ||||| */
	reserveAmount, _ := client.Cmd("LPOP", string3).Float64()
	numStocks := int(math.Floor(reserveAmount / amount))

	sellStr := symbol + ":OWNED"

	if numStocks > 0 {
		client.Cmd("HINCRBYFLOAT", username, sellStr, -numStocks)
	}
	//getBAL2 := getBalance(client, username)
	//fmt.Println("NEWBalance:", getBAL2)
	client.Cmd("LPUSH", string3, reserveAmount)
	client.Cmd("LPUSH", string3, amount)

}
func redisCANCEL_SET_SELL(client *redis.Client, username string, symbol string) {
	fmt.Println("-----CANCEL_SET_SELL-----")
	/* get length of stack */
	string3 := symbol + ":SELL:" + username
	stackLength, _ := client.Cmd("LLEN", string3).Int()
	fmt.Println("Stack length:", stackLength)

	/* if even number of items on stack
	refund second pop
	*/
	stringy := symbol + ":OWNED"
	if stackLength%2 == 0 && stackLength >= 2 {
		price, _ := client.Cmd("LPOP", string3).Float64()
		amount, _ := client.Cmd("LPOP", string3).Float64()
		refund := int(math.Floor(amount / price))

		client.Cmd("HINCRBYFLOAT", username, stringy, refund)

		//getBAL2 := getBalance(client, username)
		//fmt.Println("NEWBalance(refund):", getBAL2)
		/* else if odd # refund first pop */
	} else if stackLength%2 == 1 {
		//client.Cmd("LPOP", string3)
		client.Cmd("LPOP", string3).Float64()
		//client.Cmd("HINCRBYFLOAT", username, "Balance", refund)
		//getBAL2 := getBalance(client, username)
		//fmt.Println("NEWBalance(refund):", getBAL2)
	}

}
