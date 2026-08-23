package repositories

import (
	"pdm-backend/models"
	"time"

	"gorm.io/gorm"
)

type SubCategoriaRepository struct {
	DB *gorm.DB
}

func NewSubCategoriaRepository(db *gorm.DB) *SubCategoriaRepository {
	return &SubCategoriaRepository{DB: db}
}

type SubCategoriasFinanzas struct {
	SubCategoriaId          uint    `json:"id_opcion"`
	SubCategoriaNombre      string  `json:"nombre_opcion"`
	SubCategoriaPresupuesto float64 `json:"presupuesto_opcion"`
}

func (r *SubCategoriaRepository) GetSubCategories(finanzaId uint) ([]SubCategoriasFinanzas, error) {

	var subCategorias []SubCategoriasFinanzas
	fechaActual := time.Now()
	mes := int(fechaActual.Month())
	anio := fechaActual.Year()

	err := r.DB.Model(models.SubCategoriaEgreso{}).Where("sub_categoria_egresos.finanzas_id = ?", finanzaId).
		Select(`
		sub_categoria_egresos.id AS sub_categoria_id, 
		sub_categoria_egresos.nombre_sub_categoria AS sub_categoria_nombre, 
		CASE 
			WHEN sub_categoria_egresos.nombre_sub_categoria = 'Ahorro' THEN meta_mensuals.monto_meta
			ELSE sub_categoria_egresos.presupuesto_mensual 
		END AS sub_categoria_presupuesto
		`).
		Joins(`LEFT JOIN meta_mensuals ON meta_mensuals.finanzas_id = sub_categoria_egresos.finanzas_id
		AND meta_mensuals.mes = ? AND meta_mensuals.anio = ?
		`, mes, anio).
		Scan(&subCategorias).Error

	if err != nil {
		return nil, err
	}

	return subCategorias, err
}

type GastosOpciones struct {
	TipoId     uint   `json:"tipo_id"`
	TipoNombre string `json:"tipo_nombre"`
}

func (r *SubCategoriaRepository) GetSubCategoriesExpensesType() ([]GastosOpciones, error) {

	var opciones []GastosOpciones

	err := r.DB.Model(models.TipoPresupuesto{}).Select("tipo_presupuestos.id AS tipo_id, tipo_presupuestos.nombre_tipo_presupuesto AS tipo_nombre").
		Scan(&opciones).Error
	if err != nil {
		return nil, err
	}

	return opciones, nil
}

type SubCategoriasLista struct {
	FinanzaId          uint    `json:"finanza_id"`
	SubCategoriaId     uint    `json:"sub_categoria_id"`
	CategoriaNombre    string  `json:"categoria_nombre"`
	SubCategoriaNombre string  `json:"sub_categoria_nombre"`
	TipoGasto          string  `json:"tipo_gasto"`
	Presupuesto        float64 `json:"presupuesto"`
	NombreUsuario      string  `json:"nombre_usuario"`
}

func (r *SubCategoriaRepository) GetSubCategoriesList(finanzaId uint) ([]SubCategoriasLista, error) {
	var subCategoria []SubCategoriasLista

	ahora := time.Now()
	mes := int(ahora.Month())
	anio := ahora.Year()

	err := r.DB.Model(models.SubCategoriaEgreso{}).Where("sub_categoria_egresos.finanzas_id = ?", finanzaId).
		Select(`sub_categoria_egresos.finanzas_id AS finanza_id,
		sub_categoria_egresos.id AS sub_categoria_id, 
		categoria_egresos.nombre_categoria AS categoria_nombre, 
		sub_categoria_egresos.nombre_sub_categoria AS sub_categoria_nombre, 
		tipo_presupuestos.nombre_tipo_presupuesto AS tipo_gasto, 
		CASE
			WHEN sub_categoria_egresos.nombre_sub_categoria = 'Ahorro' THEN meta_mensuals.monto_meta
			ELSE sub_categoria_egresos.presupuesto_mensual
		END AS presupuesto,
		users.nombre AS nombre_usuario`).
		Joins("LEFT JOIN categoria_egresos ON sub_categoria_egresos.categoria_egreso_id = categoria_egresos.id").
		Joins("LEFT JOIN meta_mensuals ON meta_mensuals.finanzas_id = sub_categoria_egresos.finanzas_id AND meta_mensuals.mes = ? AND meta_mensuals.anio = ?", mes, anio).
		Joins("LEFT JOIN tipo_presupuestos ON sub_categoria_egresos.tipo_presupuesto_id = tipo_presupuestos.id").
		Joins("LEFT JOIN users ON sub_categoria_egresos.user_id = users.id").
		Scan(&subCategoria).Error

	if err != nil {
		return nil, err
	}

	return subCategoria, nil
}

func (r *SubCategoriaRepository) CreateSubCategory(subCategoria *models.SubCategoriaEgreso) error {
	return r.DB.Create(&subCategoria).Error
}

func (r *SubCategoriaRepository) GetSubCategoryById(id *uint) (*models.SubCategoriaEgreso, error) {

	var subCategoria models.SubCategoriaEgreso

	if err := r.DB.First(&subCategoria, id).Error; err != nil {
		return nil, err
	}

	return &subCategoria, nil
}

type SubCategoriaResponse struct {
	CategoriaId        uint    `json:"categoria_id"`
	NombreSubCategoria string  `json:"nombre_sub_categoria"`
	TipoGastoId        uint    `json:"tipo_gasto_id"`
	Presupuesto        float64 `json:"presupuesto"`
}

func (r *SubCategoriaRepository) GetSubCategory(id *uint) (*SubCategoriaResponse, error) {

	var subCategoria SubCategoriaResponse

	fechaActual := time.Now()
	mes := int(fechaActual.Month())
	anio := fechaActual.Year()

	tx := r.DB.Model(models.SubCategoriaEgreso{}).Where("sub_categoria_egresos.id = ?", id).
		Select(`
		sub_categoria_egresos.categoria_egreso_id AS categoria_id, 
		sub_categoria_egresos.nombre_sub_categoria AS nombre_sub_categoria, 
		sub_categoria_egresos.tipo_presupuesto_id AS tipo_gasto_id, 
		CASE
			WHEN sub_categoria_egresos.nombre_sub_categoria = 'Ahorro' THEN meta_mensuals.monto_meta
			ELSE sub_categoria_egresos.presupuesto_mensual
		END AS presupuesto
		`).
		Joins("LEFT JOIN meta_mensuals ON meta_mensuals.finanzas_id = sub_categoria_egresos.finanzas_id AND meta_mensuals.mes = ? AND meta_mensuals.anio = ?", mes, anio).
		Scan(&subCategoria)

	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	if err := tx.Error; err != nil {
		return nil, err
	}

	return &subCategoria, nil
}

func (r *SubCategoriaRepository) UpdateSubCategory(subCategoria *models.SubCategoriaEgreso) error {
	return r.DB.Save(&subCategoria).Error
}
