package mesh

type Vec3 [3]float64
type Vec2 [2]float64
type Data struct {
	Origin    Vec3
	Positions []Vec3
	Indices   []uint32
	Normals   []Vec3
	UVs       []Vec2
}
