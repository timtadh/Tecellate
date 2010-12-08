package main

import (
	"fmt"
	"easynet"
	"json"
	"net"
	"time"
	"ttypes"
)

type CompletionNotification struct {
	Identifier int
	LastProcessedTurn int
}

type Request struct {
	Identifier int
	Turn int
	Command string
}

type RespondNodeInfo struct {
	Identifier int
	Turn int
	BotData []ttypes.BotInfo
}

func listenForMaster(connectionToMaster *net.TCPConn) {
	msg, err := easynet.ReceiveFromWithError(connectionToMaster)
	if err != nil {
		killChildren()
		fmt.Printf("%d apparently was not the primary\n", config.Identifier)
		fmt.Printf("%d error seen was: %v\n", config.Identifier, err)
	} else {
		if string(msg) == "begin" {
			fmt.Printf("%d is primary\n", config.Identifier)
			primary = true
			broadcastValid()
		}
	}
}

func listenForPeer() {
	fmt.Printf("%d serving requests\n", config.Identifier)
	for data := range(listenServe) {
		//Sometimes requests will be stuck together. Here I am separating them.
		//A crappy and hopefully temporary fix.
		splitPoint := 0
		for i := 1; i < len(data); i++ {
			if data[i-1] == "}"[0] && data[i] == "{"[0] {
				splitPoint = i
				handleRequest(data[0:splitPoint])
				handleRequest(data[splitPoint:len(data)])
				break
			}
		}
		if splitPoint == 0 {
			handleRequest(data)
		}
	}
}

func handleRequest(data []uint8) {
	r := new(Request)
	err := json.Unmarshal(data, r)
	easynet.DieIfError(err, "JSON error")
	switch {
	case r.Command == "Begin" && primary == false && waitingForStart == true:
		fmt.Printf("%d handle Begin from %d\n", config.Identifier, r.Identifier)
		waitingForStart = false
		go processNodes()
	case r.Command == "GetNodes":
		fmt.Printf("%d handle GetNodes from %d\n", config.Identifier, r.Identifier)
		for respondingToRequestsFor < r.Turn {
			fmt.Printf("%d not ready for GetNodes\n", config.Identifier)
			time.Sleep(100000)
		}
		fmt.Printf("%d ready for GetNodes\n", config.Identifier)
		info := new(RespondNodeInfo)
		info.Identifier = config.Identifier
		info.Turn = respondingToRequestsFor
		info.BotData = botInfosForNeighbor(r.Identifier)
		infoString, err := json.Marshal(info)
		easynet.DieIfError(err, "JSON marshal error")
		adjsServe[r.Identifier].Write(infoString)
		fmt.Printf("%d sent GetNodes response to %d\n", config.Identifier, r.Identifier)
	}
}

func processNodes() {
	fmt.Printf("%d processing nodes\n", config.Identifier)
	for i := 0; i < config.NumTurns; i++ {
		respondingToRequestsFor = i
		fmt.Printf("%d starting turn %d\n", config.Identifier, i)
		
		otherInfos := make([]ttypes.BotInfo, len(botStates), len(botStates)*len(adjsServe))
		
		//Copy all infos from botStates into otherInfos
		for i, s := range(botStates) {
			otherInfos[i] = s.Info
		}
		
		//Get updates from neighbors
		for j, conn := range(adjsRequest) {
			fmt.Printf("%d turn %d, request neighbor %d\n", config.Identifier, i, j)
			r := new(Request)
			r.Identifier = config.Identifier
			r.Turn = respondingToRequestsFor
			r.Command = "GetNodes"
			
			rData, err := json.Marshal(r)
			easynet.DieIfError(err, "JSON marshal error")
			conn.Write(rData)
			
			info := new(RespondNodeInfo)
			err = json.Unmarshal(easynet.ReceiveFrom(conn), info)
			easynet.DieIfError(err, "JSON unmarshal error")
			
			otherInfos = append(otherInfos, info.BotData...)
		}
		for botNum, s := range(botStates) {
			req := new(ttypes.BotMoveRequest)
			req.Terrain = config.Terrain
			req.OtherBots = otherInfos
			req.Messages = nil
			req.YourX = s.Info.X
			req.YourY = s.Info.Y
			req.Kill = false
			easynet.SendJson(s.Conn, req)
			
			rsp := new(ttypes.BotMoveResponse)
			easynet.ReceiveJson(s.Conn, rsp)
			switch {
			case rsp.MoveDirection == "left":
				if otherInfos[botNum].X > 0 {
					fmt.Printf("%d moving bot %d left\n", config.Identifier, botNum)
					otherInfos[botNum].X -= 1
				}
			}
		}
		//Copy new data back into botStates.
		//RACE CONDITION: respondingToRequestsFor may be behind this
		//by one turn, so some coords may get the wrong botInfos.
		//Fix: keep a dictionary mapping turn -> info set.
		//Discard past 2 turns.
		for i, _ := range(botStates) {
			botStates[i].Info = otherInfos[i]
		}
	}
	complete <- true
}

func broadcastValid() {
	note := new(Request)
	note.Identifier = config.Identifier
	note.Turn = respondingToRequestsFor
	note.Command = "Begin"
	data, err := json.Marshal(note)
	easynet.DieIfError(err, "JSON marshal error")
	
	for i, conn := range(adjsRequest) {
		fmt.Printf("%d broadcasting to %d\n", config.Identifier, i)
		conn.Write(data)
	}
	waitingForStart = false
	go processNodes()
	
	time.Sleep(10000000)
	go listenForPeer()
}
