package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

// jobHost unlocks the wallet, mines some currency, and starts a host offering
// storage to the ant farm.
func (j *JobRunner) jobHost() {
	err := j.client.Post("/wallet/unlock", fmt.Sprintf("encryptionpassword=%s&dictionary=%s", j.walletPassword, "english"), nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR: %v\n", j.siaDirectory, err)
		return
	}

	err = j.client.Get("/miner/start", nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR: %v\n", j.siaDirectory, err)
		return
	}

	// Mine at least 50,000 SC
	desiredbalance := types.NewCurrency64(50000).Mul(types.SiacoinPrecision)
	success := false
	for start := time.Now(); time.Since(start) < 5*time.Minute; time.Sleep(time.Second) {
		var walletInfo api.WalletGET
		err = j.client.Get("/wallet", &walletInfo)
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		if walletInfo.ConfirmedSiacoinBalance.Cmp(desiredbalance) > 0 {
			success = true
			break
		}
	}
	if !success {
		log.Printf("[%v jobHost ERROR]: timeout: could not mine enough currency after 5 minutes\n", j.siaDirectory)
		return
	}

	// Create a temporary folder for hosting
	hostdir, err := ioutil.TempDir("", "hostdata")
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}
	defer os.RemoveAll(hostdir)

	// Add the storage folder.
	err = j.client.Post("/host/storage/folders/add", fmt.Sprintf("path=%s&size=30000000000"), nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Announce the host to the network
	err = j.client.Post("/host/announce", "", nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Accept contracts
	err = j.client.Post("/host", "acceptingcontracts=true", nil)
	if err != nil {
		log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
		return
	}

	// Poll the API for host settings, logging them out with `INFO` tags.  If
	// `StorageRevenue` decreases, log an ERROR.
	maxRevenue := types.NewCurrency64(0)
	for {
		var hostInfo api.HostGET
		err = j.client.Get("/host", &hostInfo)
		if err != nil {
			log.Printf("[%v jobHost ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		log.Printf("[%v jobHost INFO]: %v", j.siaDirectory, hostInfo.NetworkMetrics)

		// Print an error if storage revenue has decreased
		if hostInfo.FinancialMetrics.StorageRevenue.Cmp(maxRevenue) > 0 {
			maxRevenue = hostInfo.FinancialMetrics.StorageRevenue
		} else {
			// Storage revenue has decreased!
			log.Printf("[%v jobHost ERROR]: StorageRevenue decreased!  was %v is now %v\n", j.siaDirectory, maxRevenue, hostInfo.FinancialMetrics.StorageRevenue)
		}

		time.Sleep(time.Second * 5)
	}
}
