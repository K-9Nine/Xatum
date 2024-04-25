package main

import (
	"bytes"
	"encoding/hex"
	"sync"
	"time"

	"xatum-reference-miner/config"
	"xatum-reference-miner/log"
	"xatum-reference-miner/xatum"
	"xatum-reference-miner/xatum/client"
	"xatum-reference-miner/xelishash"
	"xatum-reference-miner/xelisutil"
)

type Job struct {
	Blob   xelisutil.BlockMiner
	Diff   uint64
	Target [32]byte
}

type Miner struct {
	client    *client.Client
	connected bool
	currentJob Job
	jobMutex   sync.RWMutex
	hashCount  float64
	startTime  time.Time
}

func NewMiner() *Miner {
	return &Miner{
		startTime: time.Now(),
	}
}

func (m *Miner) clientHandler() {
	for {
		cl, err := client.NewClient("127.0.0.1:6969")
		if err != nil {
			log.Errf("%v", err)
			time.Sleep(time.Second)
			continue
		}
		m.client = cl

		err = cl.Send(xatum.PacketC2S_Handshake, xatum.C2S_Handshake{
			Addr:  config.ADDRESS,
			Work:  "x",
			Agent: "XelMiner ALPHA",
			Algos: []string{config.ALGO},
		})
		if err != nil {
			log.Err(err)
			time.Sleep(time.Second)
			continue
		}

		go m.readJobs(cl.Jobs)
		cl.Connect()

		time.Sleep(time.Second)
	}
}

func (m *Miner) readJobs(clJobs chan xatum.S2C_Job) {
	for {
		job, ok := <-clJobs
		if !ok {
			return
		}

		m.jobMutex.Lock()
		m.connected = true
		m.currentJob = Job{
			Blob:   xelisutil.BlockMiner(job.Blob),
			Diff:   job.Diff,
			Target: xelisutil.GetTargetBytes(job.Diff),
		}
		m.jobMutex.Unlock()

		log.Infof("new job: diff %d, blob %x", job.Diff, job.Blob)
	}
}

func (m *Miner) logHashrate() {
	for {
		m.jobMutex.Lock()
		elapsed := time.Since(m.startTime).Seconds()
		if elapsed == 0 {
			elapsed = 1 // prevent division by zero
		}
		log.Infof("hashrate: %.0f", m.hashCount/elapsed)
		m.hashCount = 0
		m.startTime = time.Now()
		m.jobMutex.Unlock()

		time.Sleep(10 * time.Second)
	}
}

func main() {
	miner := NewMiner()
	for i := 0; i < 4; i++ {
		go miner.miningThread()
	}
	go miner.logHashrate()
	miner.clientHandler()
}
