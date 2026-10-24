package notebook

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
)

type SharingMember struct {
	UserId    valobj.Uid
	Email     string
	Nickname  string
	AvatarURL string
	Role      vo.Role
}

type GetNotebookSharingResult struct {
	Visibility vo.Visibility    // notebook的可见性
	Members    []*SharingMember // 被显式授权notebook访问的member详情
}

type GetNotebookSharingHandler struct {
	*baseHandler
	memberRepo  notebookrepo.MemberRepository
	userRepo    identityrepo.UserRepository
	objectStore adapter.ObjectPublicURLer
}

func NewGetNotebookSharingHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	memberRepo notebookrepo.MemberRepository,
	userRepo identityrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *GetNotebookSharingHandler {
	return &GetNotebookSharingHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo),
		memberRepo:  memberRepo,
		userRepo:    userRepo,
		objectStore: objectStore,
	}
}

// 查询notebook的分享情况
func (h *GetNotebookSharingHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
) (*GetNotebookSharingResult, error) {
	return withNotebookAccess(h.baseHandler, ctx, notebookId, accessManageSharing, h.getSharing)
}

func (h *GetNotebookSharingHandler) getSharing(
	ctx context.Context,
	access *notebookentity.Access,
) (*GetNotebookSharingResult, error) {
	notebookId := access.Notebook.Id

	members, err := h.memberRepo.ListByNotebookId(ctx, notebookId)
	if err != nil {
		return nil, errors.WithMessage(err, "list notebook members failed")
	}

	userIds := make([]valobj.Uid, 0, len(members))
	for _, member := range members {
		userIds = append(userIds, member.UserId)
	}
	userIds = pkgslices.Unique(userIds)

	users, err := h.userRepo.BatchGetById(ctx, userIds)
	if err != nil {
		return nil, errors.WithMessage(err, "get notebook member users failed")
	}

	userMap := make(map[valobj.Uid]*identityentity.User, len(users))
	for _, user := range users {
		userMap[user.Id] = user
	}

	results := make([]*SharingMember, 0, len(members))
	for _, member := range members {
		result := &SharingMember{
			UserId: member.UserId,
			Role:   member.Role,
		}
		if user, ok := userMap[member.UserId]; ok {
			result.Email = user.Email
			result.Nickname = user.Nickname
			result.AvatarURL = h.avatarURL(ctx, user.Avatar)
		}
		results = append(results, result)
	}

	return &GetNotebookSharingResult{
		Visibility: access.Notebook.Visibility,
		Members:    results,
	}, nil
}

func (h *GetNotebookSharingHandler) avatarURL(ctx context.Context, avatar valobj.StoreKey) string {
	if !avatar.Valid() {
		return ""
	}

	url, err := h.objectStore.PublicURL(ctx, avatar)
	if err != nil {
		slog.WarnContext(ctx, "resolve avatar public url failed",
			slog.String("avatar", avatar.String()),
			slog.Any("err", err),
		)
		return ""
	}

	return url
}
