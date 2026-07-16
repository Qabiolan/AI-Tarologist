package tarot

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
)

type CardInfo struct {
	Name   string `json:"name"`
	Number string `json:"number"`
	Symbol string `json:"symbol"`
	Image  string `json:"image"`
}

type TarotDeck struct {
	MajorArcana map[string]CardInfo `json:"major_arcana"`
	Wands       map[string]CardInfo `json:"wands"`
	Cups        map[string]CardInfo `json:"cups"`
	Swords      map[string]CardInfo `json:"swords"`
	Pentacles   map[string]CardInfo `json:"pentacles"`
}

var deck *TarotDeck

func getBasePath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(b)))
}

func LoadDeck() error {
	basePath := getBasePath()
	jsonPath := filepath.Join(basePath, "tarot_cards", "cards.json")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read tarot deck: %w", err)
	}

	deck = &TarotDeck{}
	if err := json.Unmarshal(data, deck); err != nil {
		return fmt.Errorf("failed to parse tarot deck: %w", err)
	}

	return nil
}

func GetRandomCards(count int) []CardInfo {
	if deck == nil {
		LoadDeck()
	}

	// Collect all cards
	var allCards []CardInfo
	for _, card := range deck.MajorArcana {
		allCards = append(allCards, card)
	}
	for _, card := range deck.Wands {
		allCards = append(allCards, card)
	}
	for _, card := range deck.Cups {
		allCards = append(allCards, card)
	}
	for _, card := range deck.Swords {
		allCards = append(allCards, card)
	}
	for _, card := range deck.Pentacles {
		allCards = append(allCards, card)
	}

	// Shuffle and pick
	rand.Shuffle(len(allCards), func(i, j int) {
		allCards[i], allCards[j] = allCards[j], allCards[i]
	})

	if count > len(allCards) {
		count = len(allCards)
	}

	return allCards[:count]
}

func GetRandomMajorArcana(count int) []CardInfo {
	if deck == nil {
		LoadDeck()
	}

	var cards []CardInfo
	for _, card := range deck.MajorArcana {
		cards = append(cards, card)
	}

	rand.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})

	if count > len(cards) {
		count = len(cards)
	}

	return cards[:count]
}

func GetCardImagePath(cardName string) string {
	basePath := getBasePath()

	// First, try to find by card name in the deck (Russian name)
	if deck != nil {
		// Search in all maps
		allMaps := []map[string]CardInfo{deck.MajorArcana, deck.Wands, deck.Cups, deck.Swords, deck.Pentacles}
		for _, cardMap := range allMaps {
			for _, card := range cardMap {
				if card.Name == cardName && card.Image != "" {
					// Found the card, return the image path
					path := filepath.Join(basePath, card.Image)
					if _, err := os.Stat(path); err == nil {
						return path
					}
				}
			}
		}
	}

	// Fallback: search in all folders by English name
	folders := []string{"major_arcana", "wands", "cups", "swords", "pentacles"}
	for _, folder := range folders {
		path := filepath.Join(basePath, "tarot_cards", folder, cardName+".jpg")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// GetCardImagePathByKey returns image path by card key (e.g., "0_The_Fool")
func GetCardImagePathByKey(cardKey string) string {
	basePath := getBasePath()

	// Search in all folders
	folders := []string{"major_arcana", "wands", "cups", "swords", "pentacles"}
	for _, folder := range folders {
		path := filepath.Join(basePath, "tarot_cards", folder, cardKey+".jpg")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// GetRandomCardsWithKeys returns random cards with their keys for image lookup
func GetRandomCardsWithKeys(count int) ([]CardInfo, []string) {
	if deck == nil {
		LoadDeck()
	}

	// Collect all cards with their keys
	type CardWithKey struct {
		Card CardInfo
		Key  string
	}

	var allCards []CardWithKey
	for key, card := range deck.MajorArcana {
		allCards = append(allCards, CardWithKey{card, key})
	}
	for key, card := range deck.Wands {
		allCards = append(allCards, CardWithKey{card, key})
	}
	for key, card := range deck.Cups {
		allCards = append(allCards, CardWithKey{card, key})
	}
	for key, card := range deck.Swords {
		allCards = append(allCards, CardWithKey{card, key})
	}
	for key, card := range deck.Pentacles {
		allCards = append(allCards, CardWithKey{card, key})
	}

	// Shuffle and pick
	rand.Shuffle(len(allCards), func(i, j int) {
		allCards[i], allCards[j] = allCards[j], allCards[i]
	})

	if count > len(allCards) {
		count = len(allCards)
	}

	var cards []CardInfo
	var keys []string
	for i := 0; i < count; i++ {
		cards = append(cards, allCards[i].Card)
		keys = append(keys, allCards[i].Key)
	}

	return cards, keys
}

func GetCardByName(name string) *CardInfo {
	if deck == nil {
		LoadDeck()
	}

	// Search in all maps
	if card, ok := deck.MajorArcana[name]; ok {
		return &card
	}
	if card, ok := deck.Wands[name]; ok {
		return &card
	}
	if card, ok := deck.Cups[name]; ok {
		return &card
	}
	if card, ok := deck.Swords[name]; ok {
		return &card
	}
	if card, ok := deck.Pentacles[name]; ok {
		return &card
	}

	return nil
}

func FormatCardsForPrompt(cards []CardInfo) string {
	result := "Выпавшие карты:\n"
	for i, card := range cards {
		result += fmt.Sprintf("%d. %s (%s) %s\n", i+1, card.Name, card.Number, card.Symbol)
	}
	return result
}
