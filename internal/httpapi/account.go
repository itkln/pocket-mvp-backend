package httpapi

import (
	"fmt"
	"io"
	"net/http"

	"pocket-mvp-backend/internal/modules/identity"
)

// The signed-in user's own account: profile, password, email and photo.

const maxAvatarBody = 2 << 20

type updateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type changeEmailRequest struct {
	CurrentPassword string `json:"current_password"`
	NewEmail        string `json:"new_email"`
}

func (api *API) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, newAuthResponse(currentUser(r)))
}

func (api *API) updateProfile(w http.ResponseWriter, r *http.Request) {
	var request updateProfileRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	updated, err := api.identity.UpdateProfile(r.Context(), currentUser(r).ID, identity.UpdateProfileInput{
		FirstName: request.FirstName,
		LastName:  request.LastName,
		Phone:     request.Phone,
	})
	if err != nil {
		api.writeError(w, r, err, authErrors)
		return
	}
	writeJSON(w, http.StatusOK, newAuthResponse(updated))
}

// changePassword keeps the current session and revokes the others.
func (api *API) changePassword(w http.ResponseWriter, r *http.Request) {
	var request changePasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	current := currentSession(r)
	err := api.identity.ChangePassword(r.Context(), identity.ChangePasswordInput{
		UserID: current.user.ID, SessionToken: current.token,
		CurrentPassword: request.CurrentPassword, NewPassword: request.NewPassword,
	})
	if err != nil {
		api.writeError(w, r, err, authErrors)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) changeEmail(w http.ResponseWriter, r *http.Request) {
	var request changeEmailRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	updated, err := api.identity.ChangeEmail(r.Context(), identity.ChangeEmailInput{
		UserID: currentUser(r).ID, CurrentPassword: request.CurrentPassword, NewEmail: request.NewEmail,
	})
	if err != nil {
		api.writeError(w, r, err, authErrors)
		return
	}
	writeJSON(w, http.StatusOK, newAuthResponse(updated))
}

func (api *API) updateAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBody+(64<<10))
	if err := r.ParseMultipartForm(maxAvatarBody); err != nil {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "avatar_too_large", "The photo must be smaller than 2 MB.")
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_avatar", "Choose a photo.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAvatarBody+1))
	if err != nil || len(data) == 0 || len(data) > maxAvatarBody {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "avatar_too_large", "The photo must be smaller than 2 MB.")
		return
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		writeAPIError(w, http.StatusUnprocessableEntity, "invalid_avatar", "Use a JPEG, PNG, or WebP image.")
		return
	}
	updated, err := api.identity.UpdateAvatar(r.Context(), currentUser(r).ID, contentType, data)
	if err != nil {
		api.writeError(w, r, err, authErrors)
		return
	}
	writeJSON(w, http.StatusOK, newAuthResponse(updated))
}

// avatar is the one account response that may be cached, privately and briefly.
func (api *API) avatar(w http.ResponseWriter, r *http.Request) {
	avatar, err := api.identity.Avatar(r.Context(), currentUser(r).ID)
	if err != nil {
		api.writeError(w, r, err, authErrors)
		return
	}
	w.Header().Set("Content-Type", avatar.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(avatar.Data)))
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(avatar.Data)
}
