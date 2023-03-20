package main

import "p2p-crawler/crawler"

func main() {
	newCrawler, err := crawler.NewCrawler(crawler.DefaultConfig())
	if err != nil {
		panic(err)
	}
	if err := newCrawler.Boot(); err != nil {
		panic(err)
	}
}
