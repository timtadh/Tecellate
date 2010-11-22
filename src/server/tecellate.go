package main

import (
	"fmt"
	"os"
	"json"
	"io/ioutil"
	"log"
	"net"
	"rand"
	"easynet"
	"ttypes"
)

func main() {
	config := loadConfig()
	connections := connectToCoordinators(config)
	coordConfigs := configureCoordinators(config)
	
	grid := simpleGrid(10, 10)
	
	for i, conn := range(connections) {
		coordConfigs[i].Terrain = *grid
		data, err := json.Marshal(coordConfigs[i])
		if (err != nil) { log.Exit(err) }
		conn.Write(data)
	}
	
	for i, conn := range(connections) {
		fmt.Printf("%d: %s\n", i+1, string(easynet.ReceiveFrom(conn)))
	}
	
	for i, conn := range(connections) {
		fmt.Printf("%d: %s\n", i+1, string(easynet.ReceiveFrom(conn)))
	}
	
	fmt.Printf("Starting first turn\n")
	connections[0].Write([]uint8("begin"))
}

func loadConfig() *ttypes.Config {
	config := new(ttypes.Config)
	
	configFile, err := os.Open("config.json", os.O_RDONLY, 0)
	if err != nil { log.Exit(err) }
	defer configFile.Close()
	
	configBytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		log.Exit(err)
	} else {
		json.Unmarshal(configBytes, config)
	}
	return config
}

func connectToCoordinators(config *ttypes.Config) ([]*net.TCPConn) {
	connections := make([]*net.TCPConn, len(config.Coords))
	for i, _ := range(connections) {
		connections[i] = easynet.Dial(config.Coords[i])
	}
	return connections
}

func configureCoordinators(config *ttypes.Config) ([]ttypes.CoordConfig) {
	coordConfigs := make([]ttypes.CoordConfig, len(config.Coords))
	for i, _ := range(coordConfigs) {
		coordConfigs[i].Identifier = i+1
		coordConfigs[i].NumTurns = config.NumTurns
	}
	for _, bot := range(config.BotDefs) {
		for i := 0; i < bot.Count; i++ {
			ix := rand.Int() % len(coordConfigs)
			newConf := ttypes.BotConf{bot.Path}
			coordConfigs[ix].BotConfs = append(coordConfigs[ix].BotConfs, newConf)
		}
	}
	for i, _ := range(coordConfigs) {
		for j, _ := range(coordConfigs) {
			if i != j {
				newAdj := ttypes.AdjacentCoord{coordConfigs[j].Identifier, config.Coords[j]};
				coordConfigs[i].AdjacentCoords = []ttypes.AdjacentCoord{newAdj}
			}
		}
	}
	return coordConfigs
}
