package favourite

import (
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) RegisterRoutes(g *route.RouterGroup) {
	g.POST("/favourites", s.CreateFavourite)
	g.GET("/favourites", s.ListFavourites)

	idGroup := g.Group("/favourites/:id")
	{
		idGroup.PATCH("", s.RenameFavourite)
		idGroup.DELETE("", s.DeleteFavourite)
		idGroup.GET("/notebooks", s.ListFavouriteNotebooks)
		idGroup.POST("/notebooks", s.AddFavouriteNotebook)
		idGroup.DELETE("/notebooks", s.RemoveFavouriteNotebooks)
	}
}
