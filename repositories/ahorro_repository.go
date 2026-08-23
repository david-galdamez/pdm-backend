package repositories

import (
	"errors"
	"pdm-backend/models"

	"gorm.io/gorm"
)

type AhorroRepository struct {
	DB *gorm.DB
}

func NewAhorroRepository(db *gorm.DB) *AhorroRepository {
	return &AhorroRepository{DB: db}
}

var meses = []string{"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}

type AhorroResponse struct {
	Mes                    int     `json:"mes"`
	NombreMes              string  `json:"nombre_mes"`
	Anio                   int     `json:"anio"`
	MetaAhorro             float64 `json:"meta_ahorro"`
	MontoAhorrado          float64 `json:"monto_ahorrado"`
	PorcentajeCumplimiento float64 `json:"porcentaje_cumplimiento"`
}

func (r *AhorroRepository) GetSavingsData(finanzaId uint, anio int) ([]AhorroResponse, error) {

	var resultados []struct {
		Mes           int
		MontoMeta     float64
		MontoAhorrado float64
	}

	err := r.DB.Model(models.MetaMensual{}).
		Select(`
			meta_mensuals.mes,
			meta_mensuals.anio,
			meta_mensuals.monto_meta,
			COALESCE(ahorro_mensuals.monto, 0) AS monto_ahorrado
		`).
		Joins(`
			LEFT JOIN ahorro_mensuals
			ON meta_mensuals.finanzas_id = ahorro_mensuals.finanzas_id
			AND meta_mensuals.mes = ahorro_mensuals.mes
			AND meta_mensuals.anio = ahorro_mensuals.anio
		`).
		Where("meta_mensuals.finanzas_id = ? AND meta_mensuals.anio = ?", finanzaId, anio).
		Order("mes ASC").
		Scan(&resultados).Error

	if err != nil {
		return nil, err
	}

	ahorroResponse := []AhorroResponse{}
	for _, r := range resultados {
		porcentaje := 0.0
		if r.MontoMeta != 0 {
			porcentaje = (r.MontoAhorrado / r.MontoMeta) * 100
		}

		ahorroResponse = append(ahorroResponse, AhorroResponse{
			Mes:                    r.Mes,
			NombreMes:              meses[r.Mes-1],
			Anio:                   anio,
			MetaAhorro:             r.MontoMeta,
			MontoAhorrado:          r.MontoAhorrado,
			PorcentajeCumplimiento: porcentaje,
		})
	}

	return ahorroResponse, nil
}

func (r *AhorroRepository) CreateOrUpdateSavingGoal(finanzaId uint, monto float64, mes, anio int) error {

	var ahorro models.MetaMensual
	err := r.DB.Model(models.MetaMensual{}).Where("finanzas_Id = ? AND anio = ? AND mes = ?", finanzaId, anio, mes).
		First(&ahorro).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nuevaMeta := models.MetaMensual{
				FinanzasID: finanzaId,
				MontoMeta:  monto,
				Mes:        mes,
				Anio:       anio,
			}

			return r.DB.Create(&nuevaMeta).Error
		}
		return err
	}

	ahorro.MontoMeta = monto

	return r.DB.Save(&ahorro).Error
}
