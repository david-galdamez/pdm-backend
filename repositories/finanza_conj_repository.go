package repositories

import (
	"errors"
	"pdm-backend/models"
	"time"

	"gorm.io/gorm"
)

type FinanzaConjRepository struct {
	DB *gorm.DB
}

func NewFinanzaConjRepository(db *gorm.DB) *FinanzaConjRepository {
	return &FinanzaConjRepository{DB: db}
}

func (r *FinanzaConjRepository) CreateConjFinance(userId uint, titulo, descripcion string) error {

	err := r.DB.Transaction(func(tx *gorm.DB) error {
		finanza := models.Finanzas{
			UserID:         userId,
			TipoFinanzasID: 2,
			Titulo:         &titulo,
			Descripcion:    &descripcion,
		}
		if err := tx.Create(&finanza).Error; err != nil {
			return err
		}

		categoria := models.CategoriaEgreso{
			FinanzasID:      finanza.ID,
			NombreCategoria: "Ahorro",
			UserID:          userId,
		}
		if err := tx.Create(&categoria).Error; err != nil {
			return err
		}

		subCategoria := models.SubCategoriaEgreso{
			FinanzasID:         finanza.ID,
			NombreSubCategoria: "Ahorro",
			TipoPresupuestoID:  3,
			CategoriaEgresoID:  categoria.ID,
			PresupuestoMensual: 0.00,
			UserID:             userId,
		}
		if err := tx.Create(&subCategoria).Error; err != nil {
			return err
		}

		finanzaConj := models.FinanzasConjunto{
			FinanzasID: finanza.ID,
			UserID:     userId,
			RolesID:    1,
			Activo:     true,
			FechaUnion: time.Now(),
		}
		if err := tx.Create(&finanzaConj).Error; err != nil {
			return err
		}

		return nil
	})

	return err
}

func (r *FinanzaConjRepository) JoinUser(userId uint, codigo string) error {

	var invitacion models.Invitaciones

	err := r.DB.Where("codigo = ?", codigo).First(&invitacion).Error
	if err != nil {
		return err
	}

	var existente models.FinanzasConjunto
	err = r.DB.Where("finanzas_id = ? AND user_id = ? AND activo = ?", invitacion.FinanzasID, userId, true).First(&existente).Error
	if err == nil {
		return errors.New("Ya perteneces a esta finanza conjunta")
	}

	if time.Now().Before(invitacion.ExpiraEn) {
		var inactivo models.FinanzasConjunto
		err = r.DB.Where("finanzas_id = ? AND user_id = ? AND activo = ?", invitacion.FinanzasID, userId, false).First(&inactivo).Error
		if err == nil {
			inactivo.Activo = true
			inactivo.FechaUnion = time.Now()

			err := r.DB.Save(&inactivo).Error
			if err != nil {
				return err
			}
		} else {
			finanzaConjunta := models.FinanzasConjunto{
				FinanzasID: invitacion.FinanzasID,
				UserID:     userId,
				RolesID:    2,
				Activo:     true,
				FechaUnion: time.Now(),
			}

			err := r.DB.Create(&finanzaConjunta).Error
			if err != nil {
				return err
			}
		}
	} else {
		return errors.New("El codigo ya ha expirado")
	}

	return nil
}

type FinancesResponse struct {
	FinanzaId     uint   `json:"finanza_id"`
	FinanzaNombre string `json:"finanza_nombre"`
	NombreAdmin   string `json:"nombre_admin"`
}

func (r *FinanzaConjRepository) GetConjFinances(userId uint) ([]FinancesResponse, error) {

	financeResponse := []FinancesResponse{}

	err := r.DB.Model(models.FinanzasConjunto{}).Where("finanzas_conjuntos.user_id = ? AND finanzas_conjuntos.activo = ?", userId, true).
		Select("finanzas.id AS finanza_id, finanzas.titulo AS finanza_nombre, admin_users.nombre AS nombre_admin").
		Joins("INNER JOIN finanzas ON finanzas.id = finanzas_conjuntos.finanzas_id").
		Joins("LEFT JOIN finanzas_conjuntos AS admin_conj ON admin_conj.finanzas_id = finanzas.id AND admin_conj.roles_id = 1").
		Joins("LEFT JOIN users AS admin_users ON admin_users.id = admin_conj.user_id").
		Scan(&financeResponse).Error
	if err != nil {
		return nil, err
	}

	return financeResponse, nil
}

type MiembrosFinanza struct {
	IdUsuario     uint   `json:"id_usuario"`
	NombreUsuario string `json:"nombre_usuario"`
	RolUsuario    uint   `json:"rol_usuario"`
}

type ConjFinancesDetails struct {
	FinanzaTitulo      string            `json:"finanza_titulo"`
	FinanzaDescripcion string            `json:"finanza_descripcion"`
	Miembros           []MiembrosFinanza `json:"finanza_miembros" gorm:"-"`
}

func (r *FinanzaConjRepository) GetConjFinancesDetails(finanzaId uint) (*ConjFinancesDetails, error) {
	var detallesFinanza ConjFinancesDetails
	var miembros []MiembrosFinanza

	err := r.DB.Model(models.Finanzas{}).
		Select("finanzas.titulo AS finanza_titulo, finanzas.descripcion AS finanza_descripcion").
		Where("finanzas.id = ?", finanzaId).
		Scan(&detallesFinanza).Error
	if err != nil {
		return nil, err
	}

	err = r.DB.Model(models.FinanzasConjunto{}).
		Select("finanzas_conjuntos.user_id AS id_usuario, users.nombre AS nombre_usuario, finanzas_conjuntos.roles_id AS rol_usuario").
		Joins("JOIN users ON users.id = finanzas_conjuntos.user_id").
		Where("finanzas_conjuntos.finanzas_id = ? AND finanzas_conjuntos.activo = ?", finanzaId, true).
		Scan(&miembros).Error
	if err != nil {
		return nil, err
	}

	detallesFinanza.Miembros = miembros
	return &detallesFinanza, nil
}

func (r *FinanzaConjRepository) LeaveConjFinance(userId, finanzaId uint) error {

	var finanzaConj models.FinanzasConjunto

	err := r.DB.Model(models.FinanzasConjunto{}).Where("finanzas_id = ? AND user_id = ?", finanzaId, userId).
		First(&finanzaConj).Error
	if err != nil {
		return err
	}

	finanzaConj.Activo = false

	return r.DB.Save(&finanzaConj).Error
}
