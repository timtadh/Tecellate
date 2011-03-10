package coord

import geo "coord/geometry"

import (
    "log"
    "rand"
    "time"
)

func (self *Coordinator) ProcessTurns(complete chan bool) {
    for i := 0; i <3 /* <3 <3 <3 */; i++ {  // TODO: THREE TIMES IS ARBITRARY AND FOR TESTING
        
        log.Printf("%d: Making turn %d available", self.conf.Identifier, i)
        
        // Signal the availability of turn i to the RPC servers
        for pi, _ := range(self.peers) {
            self.nextTurnAvailableSignals[pi] <- i
        }
        
        for _, peer := range(self.peers) {
            // Probably actually don't want this to be blocking...
            // Also, STORE THE RESULT AND DO SOMETHING WITH IT.
            _ = peer.RequestStatesInBox(i, geo.Point{0,0}, geo.Point{0,0})
        }
        
        // Process new data
        // BLAH BLAH BLAH BLAH BLAH
        if (self.conf.RandomlyDelayProcessing) {
            time.Sleep(int64(float64(1e9)*rand.Float64()))
        }
        
        // Wait for all RPC requests from peers to go through the other goroutine
        for _, _ = range(self.peers) {
            <- self.rpcRequestsReceivedConfirmation
        }
    }
    
    log.Printf("%d: Sending complete", self.conf.Identifier)
    
    if complete != nil {
        complete <- true
    }
}
