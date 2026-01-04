package main

import (
	"github.com/samber/lo"
)

type productCategoryItem struct {
	Emoji string
	Label string
}

var (
	// https://github.com/joonas-fi/shopping-list-manager/issues/3#issuecomment-3694679026
	productCategories = []productCategoryItem{
		{"❓", "Other"}, // offer this first to have it as default if no sane option is selected
		{"🥕", "Produce (Fruits & Vegetables)"},
		{"🥩", "Meat & Seafood"},
		{"🧀", "Deli"},
		{"🥚", "Dairy & Eggs"},
		{"🍞", "Bakery / Bread"},
		{"🧺", "Pantry / Dry Goods"},
		{"🥫", "Canned & Jarred"},
		{"🎂", "Baking Supplies"},
		{"🥣", "Breakfast (cereal, oatmeal, spreads)"},
		{"🍿", "Snacks"},
		{"🥤", "Beverages"},
		{"🧊", "Frozen Foods"},
		{"🧂", "Condiments & Sauces"},
		{"🌶️", "Spices & Seasonings"},
		{"🪣", "Household / Cleaning"},
		{"🧻", "Paper Goods (toilet paper, napkins, towels)"},
		{"🪥", "Personal Care / Health"},
		{"👶", "Baby"},
		{"🐾", "Pet"},
		{"🍷", "Alcohol"},
	}
	productCategoriesLabelsOnly = lo.Map(productCategories, func(item productCategoryItem, _ int) string { return item.Label })
)

func resolveProductCategory(label string) (*productCategoryItem, int) {
	for idx, cat := range productCategories {
		if cat.Label == label {
			return &cat, idx
		}
	}
	return nil, -1
}
