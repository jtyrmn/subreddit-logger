package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/jtyrmn/reddit-votewatch/database"
	"github.com/jtyrmn/reddit-votewatch/reddit"
)

func main() {
	//load env variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}

	client := reddit.Connect()
	fmt.Println(client)

	fmt.Println("\ngetting data...")
	data, err := client.GetNewestPosts("dwarffortress", 5)
	if err != nil {
		panic(err)
	}
	for _, p := range data {
		fmt.Println(p)
	}
	fmt.Println(len(data), "posts recieved")

	fmt.Println("\nre-requesting data...")
	IDs := make([]reddit.Fullname, len(data))
	for i := range IDs {
		IDs[i] = data[i].FullId()
	}

	data2, err := client.FetchPosts(IDs)
	if err != nil {
		panic(err)
	}

	for _, post := range *data2 {
		//fmt.Printf("%s: %s\n", ID, post.Title)
		fmt.Println(post)
	}

	fmt.Println("\nconnecting to db...")
	conn, err := database.Connect()
	if err != nil {
		panic(err)
	}

	fmt.Println("sending to db...")
	conn.SaveListings(*data2)

	fmt.Println("recieving from db...")
	data3 := make(reddit.ContentGroup)
	conn.RecieveListings(data3)

	for _, val := range data3 {
		fmt.Println(val)
	}

	fmt.Println("\nupdating records...")
	conn.RecordNewData(*data2)
}
