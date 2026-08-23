package repositories

import (
	"errors"
	"pdm-backend/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FinanzaRepository struct {
	DB *gorm.DB
}

func NewFinanzaRepository(db *gorm.DB) *FinanzaRepository {
	return &FinanzaRepository{DB: db}
}

func SumarMonto(db *gorm.DB, modelo interface{}, finanzaId uint, tipo int, inicio, final time.Time) (float64, error) {
	var total float64

	err := db.Model(modelo).
		Where("finanzas_id = ? AND tipo_registro_id = ? AND fecha_registro >= ? AND fecha_registro < ?", finanzaId, tipo, inicio, final).
		Select("COALESCE(SUM(monto), 0)").Scan(&total).Error

	return total, err
}

type Resumen struct {
	IngresosTotales float64 `json:"ingresos_totales" gorm:"column:ingresos_totales"`
	EgresosTotales  float64 `json:"egresos_totales" gorm:"column:egresos_totales"`
	Diferencia      float64 `json:"diferencia" gorm:"-"`
}

func (r *FinanzaRepository) GetFinanceSummary(finanzaId uint, inicio, final time.Time) (gin.H, error) {

	var resumen Resumen
	err := r.DB.Model(&models.Transacciones{}).
		Select("SUM(CASE WHEN tipo_registro_id = 1 THEN monto ELSE 0 END) AS ingresos_totales, SUM(CASE WHEN tipo_registro_id = 2 THEN monto ELSE 0 END) AS egresos_totales").
		Where("finanzas_id = ? AND fecha_registro >= ? AND fecha_registro < ? AND deleted_at IS NULL", finanzaId, inicio, final).
		Scan(&resumen).Error
	if err != nil {
		return nil, err
	}

	resumen.Diferencia = resumen.IngresosTotales - resumen.EgresosTotales

	return gin.H{
		"ingresos_totales": resumen.IngresosTotales,
		"egresos_totales":  resumen.EgresosTotales,
		"diferencia":       resumen.Diferencia,
	}, nil
}

func (r *FinanzaRepository) GetEgresoSummary(finanzaId uint, inicio, final time.Time) (gin.H, error) {

	var egresosTotales, presupuestoMensual float64
	errCh := make(chan error, 2)

	go func() {
		err := r.DB.Model(models.SubCategoriaEgreso{}).
			Where("finanzas_id = ?", finanzaId).
			Select("COALESCE(SUM(presupuesto_mensual), 0)").
			Scan(&presupuestoMensual).Error

		errCh <- err
	}()

	go func() {
		monto, err := SumarMonto(r.DB, models.Transacciones{}, finanzaId, 2, inicio, final)

		if err == nil {
			egresosTotales = monto
		}

		errCh <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	variacion := presupuestoMensual - egresosTotales

	return gin.H{
		"presupuesto_mensual": presupuestoMensual,
		"consumo_mensual":     egresosTotales,
		"variacion_mensual":   variacion,
	}, nil
}

func (r *FinanzaRepository) GetSavingsSummary(finanzaId uint, mes, anio int) (gin.H, error) {
	var metaAhorro float64
	var ahorroGuardado float64

	var meta models.MetaMensual
	errCh := make(chan error, 2)

	go func() {
		err := r.DB.Model(models.MetaMensual{}).Where("finanzas_id = ? AND mes = ? AND anio = ?", finanzaId, mes, anio).
			First(&meta).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	go func() {
		err := r.DB.Model(models.AhorroMensual{}).
			Where("finanzas_id = ? AND mes = ? AND anio = ?", finanzaId, mes, anio).
			Select("monto").Scan(&ahorroGuardado).Error
		errCh <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	metaAhorro = meta.MontoMeta
	porcentajeAhorro := 0.0

	if metaAhorro != 0 {
		porcentajeAhorro = (ahorroGuardado * 100) / metaAhorro
	}

	return gin.H{
		"meta":                metaAhorro,
		"acumulado":           ahorroGuardado,
		"progreso_porcentaje": porcentajeAhorro,
	}, nil
}

func (r FinanzaRepository) GetDashboardSummary(finanzaId uint, inicioMes, finMes time.Time) (gin.H, error) {

	var resumenFinanciero gin.H
	var resumenEgresos gin.H
	var resumenAhorro gin.H

	errCh := make(chan error, 3)

	go func() {
		resumen, err := r.GetFinanceSummary(finanzaId, inicioMes, finMes)
		if err == nil {
			resumenFinanciero = resumen
		}
		errCh <- err

	}()

	go func() {
		resumen, err := r.GetEgresoSummary(finanzaId, inicioMes, finMes)
		if err == nil {
			resumenEgresos = resumen
		}
		errCh <- err
	}()

	go func() {
		resumen, err := r.GetSavingsSummary(finanzaId, int(inicioMes.Month()), inicioMes.Year())
		if err == nil {
			resumenAhorro = resumen
		}
		errCh <- err
	}()

	for i := 0; i < 3; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	return gin.H{
		"resumen_financiero": resumenFinanciero,
		"resumen_egresos":    resumenEgresos,
		"resumen_ahorros":    resumenAhorro,
	}, nil

}

type DashboardData struct {
	FinanzaId        uint    `json:"finanza_id"`
	CategoriaId      uint    `json:"-"`
	CategoriaNombre  string  `json:"categoria_nombre"`
	TotalPresupuesto float64 `json:"total_presupuesto"`
	Gasto            float64 `json:"gasto"`
	Diferencia       float64 `json:"diferencia"`
}

func (r *FinanzaRepository) GetDataSummary(inicioMes, finMes time.Time, finanzaId uint) ([]DashboardData, error) {

	var resultados []DashboardData

	err := r.DB.Model(models.CategoriaEgreso{}).Where("categoria_egresos.finanzas_id = ?", finanzaId).
		Select("categoria_egresos.id AS categoria_id, categoria_egresos.nombre_categoria AS categoria_nombre, COALESCE(SUM(sub_categoria_egresos.presupuesto_mensual), 0) AS total_presupuesto").
		Joins("LEFT JOIN sub_categoria_egresos ON sub_categoria_egresos.categoria_egreso_id = categoria_egresos.id").
		Group("categoria_egresos.id, categoria_egresos.nombre_categoria").
		Order("categoria_egresos.nombre_categoria").
		Scan(&resultados).Error
	if err != nil {
		return nil, err
	}

	mes := int(inicioMes.Month())
	anio := inicioMes.Year()

	for index := range resultados {

		resultados[index].FinanzaId = finanzaId

		if resultados[index].CategoriaNombre == "Ahorro" {
			var meta models.MetaMensual
			err := r.DB.Model(models.MetaMensual{}).
				Where("finanzas_id = ? AND mes = ? AND anio = ?", finanzaId, mes, anio).
				First(&meta).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}

			var ahorroMensual models.AhorroMensual
			err = r.DB.Model(models.AhorroMensual{}).
				Where("finanzas_id = ? AND mes = ? AND anio = ?", finanzaId, mes, anio).
				First(&ahorroMensual).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}

			resultados[index].TotalPresupuesto = meta.MontoMeta
			resultados[index].Gasto = ahorroMensual.Monto
			resultados[index].Diferencia = meta.MontoMeta - ahorroMensual.Monto

			continue
		}

		var totalGasto float64

		err := r.DB.Model(models.Transacciones{}).Where("finanzas_id = ? AND tipo_registro_id = ? AND fecha_registro >= ? AND fecha_registro < ? AND categoria_egreso_id = ?", finanzaId, 2, inicioMes, finMes, resultados[index].CategoriaId).Select("COALESCE(SUM(monto), 0)").Scan(&totalGasto).Error

		if err != nil {
			return nil, err
		}

		diferencia := resultados[index].TotalPresupuesto - totalGasto

		resultados[index].Gasto = totalGasto
		resultados[index].Diferencia = diferencia
	}

	return resultados, nil
}
