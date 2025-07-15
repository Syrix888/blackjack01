
package main

import (
    "encoding/json"
    "log"
    "math/rand"
    "net/http"
    "strings"
    "sync"
    "time"
    "fmt"
)

type Card struct {
    Suit  string `json:"suit"`
    Value string `json:"value"`
}

type Hand struct {
    Cards []Card `json:"cards"`
    Done  bool   `json:"done"` // Has the player chosen to Stand or Busted
    Busted bool  `json:"busted"`
}

type GameState struct {
    Deck     []Card          `json:"-"`
    Dealer   Hand            `json:"dealer"`
    Players  []Hand          `json:"players"`
    Results  []string        `json:"results"` // "Win", "Lose", "Bust", "Playing"
    Turn     int             `json:"turn"`    // Player index (0-2), 3 means dealer
    Status   string          `json:"status"`  // "playing", "finished"
}

var (
    games = make(map[string]*GameState)
    mu    sync.Mutex
    suits = []string{"S", "H", "D", "C"} // <-- suit as alphabets
    values = []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}
)

func newDeck() []Card {
    deck := make([]Card, 0, 52)
    for _, suit := range suits {
        for _, value := range values {
            deck = append(deck, Card{Suit: suit, Value: value})
        }
    }
    rand.Seed(time.Now().UnixNano())
    rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
    return deck
}

func handValue(hand Hand) (val int, soft bool) {
    aceCount := 0
    val = 0
    for _, card := range hand.Cards {
        switch card.Value {
        case "A":
            val += 11
            aceCount++
        case "K", "Q", "J":
            val += 10
        default:
            v := 0
            fmt.Sscanf(card.Value, "%d", &v)
            val += v
        }
    }
    soft = false
    for val > 21 && aceCount > 0 {
        val -= 10
        aceCount--
        soft = true
    }
    return
}

func isBusted(hand Hand) bool {
    v, _ := handValue(hand)
    return v > 21
}

func dealCard(gs *GameState, hand *Hand) {
    if len(gs.Deck) == 0 {
        gs.Deck = newDeck()
    }
    card := gs.Deck[0]
    gs.Deck = gs.Deck[1:]
    hand.Cards = append(hand.Cards, card)
}

func startGame(roomid string) *GameState {
    gs := &GameState{
        Deck:    newDeck(),
        Players: make([]Hand, 3),
        Dealer:  Hand{},
        Results: make([]string, 3),
        Turn:    0,
        Status:  "playing",
    }
    // Deal 2 cards to each player and dealer
    for i := 0; i < 2; i++ {
        for p := range gs.Players {
            dealCard(gs, &gs.Players[p])
        }
        dealCard(gs, &gs.Dealer)
    }
    for i := range gs.Results {
        gs.Results[i] = "Playing"
    }
    return gs
}

func gameResult(gs *GameState) {
    dVal, _ := handValue(gs.Dealer)
    for i := range gs.Players {
        pVal, _ := handValue(gs.Players[i])
        if gs.Players[i].Busted {
            gs.Results[i] = "Bust"
        } else if dVal > 21 || pVal > dVal {
            gs.Results[i] = "Win"
        } else if pVal < dVal {
            gs.Results[i] = "Lose"
        } else {
            gs.Results[i] = "Push"
        }
    }
}

func handleStart(w http.ResponseWriter, r *http.Request) {
    // POST /game/start/<gameroomid>
    seg := strings.Split(r.URL.Path, "/")
    if len(seg) < 4 {
        http.Error(w, "Missing gameroomid", 400)
        return
    }
    roomid := seg[3]
    mu.Lock()
    defer mu.Unlock()
    gs := startGame(roomid)
    games[roomid] = gs
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(gs)
}

func handleState(w http.ResponseWriter, r *http.Request) {
    // GET /game/state/<gameroomid>
    seg := strings.Split(r.URL.Path, "/")
    if len(seg) < 4 {
        http.Error(w, "Missing gameroomid", 400)
        return
    }
    roomid := seg[3]
    mu.Lock()
    gs, ok := games[roomid]
    mu.Unlock()
    if !ok {
        http.Error(w, "Game not found", 404)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(gs)
}

type ActionRequest struct {
    Player  int    `json:"player"` // 0,1,2
    Action  string `json:"action"` // "hit" or "stand"
}

func handleAction(w http.ResponseWriter, r *http.Request) {
    // POST /game/action/<gameroomid>
    seg := strings.Split(r.URL.Path, "/")
    if len(seg) < 4 {
        http.Error(w, "Missing gameroomid", 400)
        return
    }
    roomid := seg[3]
    mu.Lock()
    gs, ok := games[roomid]
    if !ok || gs.Status != "playing" {
        mu.Unlock()
        http.Error(w, "Game not found or not in playing state", 400)
        return
    }
    var req ActionRequest
    err := json.NewDecoder(r.Body).Decode(&req)
    if err != nil {
        mu.Unlock()
        http.Error(w, "Invalid input", 400)
        return
    }
    if req.Player < 0 || req.Player > 2 {
        mu.Unlock()
        http.Error(w, "Invalid player", 400)
        return
    }
    player := &gs.Players[req.Player]
    if player.Done {
        mu.Unlock()
        http.Error(w, "Player already finished", 400)
        return
    }
    if req.Action == "hit" {
        dealCard(gs, player)
        if isBusted(*player) {
            player.Busted = true
            player.Done = true
        }
    } else if req.Action == "stand" {
        player.Done = true
    } else {
        mu.Unlock()
        http.Error(w, "Unknown action", 400)
        return
    }
    // Advance turn to next unfinished player or dealer
    allDone := true
    for i := range gs.Players {
        if !gs.Players[i].Done {
            gs.Turn = i
            allDone = false
            break
        }
    }
    if allDone {
        // Dealer's turn
        for {
            dVal, _ := handValue(gs.Dealer)
            if dVal < 17 {
                dealCard(gs, &gs.Dealer)
            } else {
                break
            }
        }
        gs.Status = "finished"
        gameResult(gs)
        gs.Turn = 3 // Dealer
    }
    mu.Unlock()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(gs)
}

func main() {
    http.HandleFunc("/game/start/", handleStart)
    http.HandleFunc("/game/state/", handleState)
    http.HandleFunc("/game/action/", handleAction)
    log.Println("Blackjack server running on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
