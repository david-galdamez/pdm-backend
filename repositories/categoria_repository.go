package repositories

import (
	"pdm-backend/models"
	"time"

	"gorm.io/gorm"
)

type CategoriaRepository struct {
	DB *gorm.DB
}

func NewCategoriaRepository(db *gorm.DB) *CategoriaRepository {
	return &CategoriaRepository{DB: db}
}

type CategoriasFinanzas struct {
	CategoriaId     uint   `json:"categoria_id"`
	CategoriaNombre string `json:"categoria_nombre"`
}

func (r *CategoriaRepository) GetCategories(finanzaId uint) ([]CategoriasFinanzas, error) {

	var categorias []CategoriasFinanzas

	err := r.DB.Model(models.CategoriaEgreso{}).Where("finanzas_id = ?", finanzaId).
		Select("categoria_egresos.id AS categoria_id, categoria_egresos.nombre_categoria AS categoria_nombre").
		Scan(&categorias).Error

	if err != nil {
		return nil, err
	}

	return categorias, err
}

type SubCategorias struct {
	Nombre      string  `json:"nombre_sub_categoria"`
	Presupuesto float64 `json:"presupuesto_sub_categoria"`
	Gasto       float64 `json:"gasto_sub_categoria"`
	Diferencia  float64 `json:"diferencia_sub_categoria"`
}

type CategoriaResumen struct {
	Presupuesto float64
	Gasto       float64
}

type JSONResponse struct {
	Presupuesto   float64         `json:"presupuesto_total"`
	Gasto         float64         `json:"gasto_total"`
	Diferencia    float64         `json:"diferencia_total"`
	SubCategorias []SubCategorias `json:"sub_categorias"`
}

func (r *CategoriaRepository) GetCategoriesData(finanzaId uint, categoriaId *uint) (*JSONResponse, error) {

	var resumen CategoriaResumen
	var subCategorias []SubCategorias
	errCh := make(chan error, 2)

	fechaActual := time.Now()
	mes := int(fechaActual.Month())
	anio := fechaActual.Year()

	baseQuery := func(tx *gorm.DB) *gorm.DB {
		q := tx.Where("sub_categoria_egresos.finanzas_id = ?", finanzaId)
		if categoriaId != nil {
			q = q.Where("sub_categoria_egresos.categoria_egreso_id = ?", *categoriaId)
		}
		return q
	}

	go func() {
		err := baseQuery(r.DB.Model(models.SubCategoriaEgreso{})).
			Select(`
			CASE
				WHEN sub_categoria_egresos.nombre_sub_categoria = 'Ahorro' THEN MAX(meta_mensuals.monto_meta)
				ELSE COALESCE(SUM(sub_categoria_egresos.presupuesto_mensual),0) 
			END AS presupuesto,
			COALESCE(SUM(transacciones.monto), 0) AS gasto`).
			Joins("LEFT JOIN meta_mensuals ON meta_mensuals.finanzas_id = sub_categoria_egresos.finanzas_id AND meta_mensuals.mes = ? AND meta_mensuals.anio = ?", mes, anio).
			Joins("LEFT JOIN transacciones on transacciones.sub_categoria_egreso_id = sub_categoria_egresos.id").
			Group("sub_categoria_egresos.id").Scan(&resumen).Error
		errCh <- err
	}()

	go func() {
		err := baseQuery(r.DB.Model(models.SubCategoriaEgreso{})).
			Select(`
			sub_categoria_egresos.nombre_sub_categoria AS nombre, 
			CASE
				WHEN sub_categoria_egresos.nombre_sub_categoria = 'Ahorro' THEN MAX(meta_mensuals.monto_meta)
				ELSE COALESCE(SUM(sub_categoria_egresos.presupuesto_mensual),0) 
			END AS presupuesto,
			COALESCE(SUM(transacciones.monto),0) AS gasto`).
			Joins("LEFT JOIN transacciones ON transacciones.sub_categoria_egreso_id = sub_categoria_egresos.id").
			Joins("LEFT JOIN meta_mensuals ON meta_mensuals.finanzas_id = sub_categoria_egresos.finanzas_id AND meta_mensuals.mes = ? AND meta_mensuals.anio = ?", mes, anio).
			Group("sub_categoria_egresos.id").Scan(&subCategorias).Error
		errCh <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	for index := range subCategorias {
		subCategorias[index].Diferencia = subCategorias[index].Presupuesto - subCategorias[index].Gasto
	}

	respuesta := JSONResponse{
		Presupuesto: resumen.Presupuesto,
		Gasto:       resumen.Gasto,
		Diferencia:  resumen.Presupuesto - resumen.Gasto,
	}

	if subCategorias == nil {
		respuesta.SubCategorias = []SubCategorias{}
	} else {
		respuesta.SubCategorias = subCategorias
	}

	return &respuesta, nil
}

func (r *CategoriaRepository) CreateCategory(categoria *models.CategoriaEgreso) error {
	return r.DB.Create(&categoria).Error
}

func (r *CategoriaRepository) GetCategoryById(id *uint) (*models.CategoriaEgreso, error) {

	var categoria models.CategoriaEgreso

	if err := r.DB.First(&categoria, id).Error; err != nil {
		return nil, err
	}

	return &categoria, nil
}

type ListaCategorias struct {
	CategoriaId     uint   `json:"categoria_id"`
	CategoriaNombre string `json:"categoria_nombre"`
	NombreUsuario   string `json:"nombre_usuario"`
}

func (r *CategoriaRepository) GetCategoriesList(finanzaId uint) ([]ListaCategorias, error) {

	var listaCategorias []ListaCategorias

	err := r.DB.Model(models.CategoriaEgreso{}).Where("finanzas_id = ?", finanzaId).
		Select("categoria_egresos.id AS categoria_id, categoria_egresos.nombre_categoria AS categoria_nombre, users.nombre AS nombre_usuario").
		Joins("LEFT JOIN users ON users.id = categoria_egresos.user_id").
		Scan(&listaCategorias).Error

	if err != nil {
		return nil, err
	}

	return listaCategorias, err
}

func (r *CategoriaRepository) UpdateCategory(categoria *models.CategoriaEgreso) error {
	return r.DB.Save(&categoria).Error
}
