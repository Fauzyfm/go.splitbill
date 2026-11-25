package service

import (
	"context"
	"fmt"
	"gosplitbill/model"
	"gosplitbill/repository"
	"gosplitbill/utils"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	authRepo repository.AuthRepository
}

func NewAuthService(authRepo repository.AuthRepository) *AuthService {
	return &AuthService{authRepo: authRepo}
}

func (s *AuthService) Register(ctx context.Context, userName string, email string, password string, confirmPassword string) (*model.User, error) {

	if userName == "" || email == "" || password == "" || confirmPassword == "" {
		return nil, fmt.Errorf("field wajib diisi")
	}

	if password != confirmPassword {
		return nil, fmt.Errorf("password dan confirm password harus sama!")
	}

	if len(password) < 8 {
		return nil, fmt.Errorf("Password minimal 8 karakter!")
	}

	checkEmail, _ := s.authRepo.GetByEmail(ctx, email)
	if checkEmail != nil {
		return nil, fmt.Errorf("email sudah pernah terdaftar!")
	}


	passHash, err := bcrypt.GenerateFromPassword([]byte (password), 12)
	if err != nil {
		return nil, fmt.Errorf("gagal memproses password!")
	}


	user := &model.User{
		Email: email,
		Password: string(passHash),
		UserName: userName,
	}


	if err := s.authRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email string, password string) (string, error) {

	exsistingEmail, err := s.authRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("Email tersebut belum terdaftar, silahkan register terlebih dahulu")
	}

	if err := bcrypt.CompareHashAndPassword([]byte (exsistingEmail.Password), []byte (password)); err != nil {
		return "", fmt.Errorf("Email ataU Password salah!")
	} 

	accsessToken, err := utils.GenerateTokenJWT(exsistingEmail)

	return accsessToken, nil

}

