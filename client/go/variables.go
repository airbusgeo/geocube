package client

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	pb "github.com/airbusgeo/geocube/client/go/pb"
	"github.com/golang/protobuf/ptypes/wrappers"
)

type ColorPoint pb.ColorPoint

type Variable struct {
	client *Client
	pb.Variable
}

type VariableInstance struct {
	*Variable
	InstanceID       string
	InstanceName     string
	InstanceMetadata map[string]string
}

// Instance returns a VariableInstance or nil if the instance does not exist
func (v *Variable) Instance(name string) *VariableInstance {
	for _, instance := range v.Instances {
		if instance.GetName() == name {
			return &VariableInstance{
				Variable:         v,
				InstanceID:       instance.GetId(),
				InstanceName:     instance.GetName(),
				InstanceMetadata: instance.GetMetadata(),
			}
		}
	}
	return nil
}

// SetName sets the name of the variable
func (v *Variable) SetName(name string) error {
	if err := v.client.UpdateVariable(v.Id, &name, nil, nil, nil, nil); err != nil {
		return grpcError(err)
	}
	v.Name = name
	return nil
}

// SetUnit sets the unit of the variable
func (v *Variable) SetUnit(unit string) error {
	if err := v.client.UpdateVariable(v.Id, nil, &unit, nil, nil, nil); err != nil {
		return grpcError(err)
	}
	v.Unit = unit
	return nil
}

// SetDescription sets the description of the variable
func (v *Variable) SetDescription(description string) error {
	if err := v.client.UpdateVariable(v.Id, nil, nil, &description, nil, nil); err != nil {
		return grpcError(err)
	}
	v.Description = description
	return nil
}

// SetPalette sets the name of the palette. Empty string removes the palette
func (v *Variable) SetPalette(palette string) error {
	if err := v.client.UpdateVariable(v.Id, nil, nil, nil, &palette, nil); err != nil {
		return grpcError(err)
	}
	v.Palette = palette
	return nil
}

// SetResamplingAlg sets the algorithm used for resampling
func (v *Variable) SetResamplingAlg(resamplingAlg string) error {
	if err := v.client.UpdateVariable(v.Id, nil, nil, nil, nil, &resamplingAlg); err != nil {
		return grpcError(err)
	}
	v.ResamplingAlg = toResampling(resamplingAlg)
	return nil
}

// ToString returns a string representation of the variable
func (v *Variable) ToString() string {
	s := v.toStringWithoutInstances() + "  Instances:\n"
	for _, instance := range v.Instances {
		s += fmt.Sprintf(
			"    Name:       %s\n"+
				"      Id:       %s\n"+
				"      Metadata: ", instance.GetName(), instance.GetId())
		appendDict(instance.GetMetadata(), &s)
	}

	return s
}

// ToString returns a string representation of the instance
func (vi *VariableInstance) ToString() string {
	s := vi.toStringWithoutInstances()
	s += fmt.Sprintf(
		"Instance %s:\n"+
			"  Id:       %s\n"+
			"  Metadata: ", vi.InstanceName, vi.InstanceID)
	appendDict(vi.InstanceMetadata, &s)

	return s
}

func (v *Variable) toStringWithoutInstances() string {
	s := fmt.Sprintf("\nVariable %s:\n"+
		"  Id:            %s\n"+
		"  Unit:          %s\n"+
		"  Description:   %s\n"+
		"  DFormat:       (%s, %f, [%f, %f])\n"+
		"  Bands:         [%s]\n"+
		"  Palette:       %s\n"+
		"  ResamplingAlg: %s\n",
		v.GetName(), v.GetId(), v.GetUnit(), v.GetDescription(),
		v.Dformat.Dtype.String(), v.Dformat.NoData, v.Dformat.MinValue, v.Dformat.MaxValue,
		strings.Join(v.GetBands(), ","), v.GetPalette(), v.GetResamplingAlg())
	return s
}

func appendDict(m map[string]string, s *string) {
	*s += "{"
	for k, v := range m {
		*s += fmt.Sprintf("\"%s: %v\" ", k, v)
	}
	*s += "}\n"
}

// instanceFromID returns a VariableInstance or nil if the instance does not exist
func (v *Variable) instanceFromID(id string) *VariableInstance {
	for _, instance := range v.Instances {
		if instance.GetId() == id {
			return &VariableInstance{
				Variable:         v,
				InstanceID:       instance.GetId(),
				InstanceName:     instance.GetName(),
				InstanceMetadata: instance.GetMetadata(),
			}
		}
	}
	return nil
}

// ToPbDFormat returns a DataFormat from the user-defined string
// Format is "datatype,nodata,min,max"
// with datatype in {"byte, uint8, uint16, uint32, int8,int16, int32, float32, float64, complex64, auto, u1, u2, u4, i1, i2, i4, f4, f8, c8"}
// with nodata, min and max as float value
func ToPbDFormat(s string) (*pb.DataFormat, error) {
	ss := strings.Split(s, ",")
	if len(ss) != 4 {
		return nil, errors.New("wrong format for dformat")
	}
	var dtype pb.DataFormat_Dtype
	switch strings.ToLower(ss[0]) {
	case "uint8", "u1", "byte":
		dtype = pb.DataFormat_UInt8
	case "uint16", "u2":
		dtype = pb.DataFormat_UInt16
	case "uint32", "u4":
		dtype = pb.DataFormat_UInt32
	case "int16", "i2":
		dtype = pb.DataFormat_Int16
	case "int32", "i4":
		dtype = pb.DataFormat_Int32
	case "float32", "f4":
		dtype = pb.DataFormat_Float32
	case "float64", "f8":
		dtype = pb.DataFormat_Float64
	case "complex64", "c8":
		dtype = pb.DataFormat_Complex64
	case "auto":
		dtype = pb.DataFormat_UNDEFINED
	default:
		return nil, errors.New("unrecognized DataType")
	}
	var nodata, minValue, maxValue float64
	var err error
	if nodata, err = strconv.ParseFloat(strings.Trim(ss[1], " "), 64); err != nil {
		return nil, err
	}
	if minValue, err = strconv.ParseFloat(strings.Trim(ss[2], " "), 64); err != nil {
		return nil, err
	}
	if maxValue, err = strconv.ParseFloat(strings.Trim(ss[3], " "), 64); err != nil {
		return nil, err
	}

	return &pb.DataFormat{
		Dtype:    dtype,
		NoData:   nodata,
		MinValue: minValue,
		MaxValue: maxValue,
	}, nil
}

func toResampling(s string) pb.Resampling {
	if val, ok := pb.Resampling_value[s]; ok {
		return pb.Resampling(val)
	}
	return pb.Resampling_UNDEFINED
}

// CreateVariable creates a variable
func (c Client) CreateVariable(name, unit, description string, dformat *pb.DataFormat, bandsName []string, palette, resamplingAlg string) (string, error) {
	resp, err := c.gcc.CreateVariable(c.ctx,
		&pb.CreateVariableRequest{
			Variable: &pb.Variable{
				Name:          name,
				Unit:          unit,
				Description:   description,
				Dformat:       dformat,
				Bands:         bandsName,
				Palette:       palette,
				ResamplingAlg: toResampling(resamplingAlg),
			}})
	if err != nil {
		return "", grpcError(err)
	}
	return resp.GetId(), nil
}

// InstantiateVariable instantiates a variable with name and metadata
func (c Client) InstantiateVariable(variableID, name string, metadata map[string]string) (string, error) {
	resp, err := c.gcc.InstantiateVariable(c.ctx,
		&pb.InstantiateVariableRequest{
			VariableId:       variableID,
			InstanceName:     name,
			InstanceMetadata: metadata,
		})
	if err != nil {
		return "", grpcError(err)
	}
	return resp.GetInstance().GetId(), nil
}

// GetVariable returns the variable that id
func (c Client) GetVariable(id string) (*Variable, error) {
	resp, err := c.gcc.GetVariable(c.ctx, &pb.GetVariableRequest{Identifier: &pb.GetVariableRequest_Id{Id: id}})
	if err != nil {
		return nil, grpcError(err)
	}
	return &Variable{client: &c, Variable: *resp.GetVariable()}, nil
}

// GetVariableFromInstanceID returns the variable & instance with that instance id
func (c Client) GetVariableFromInstanceID(id string) (*VariableInstance, error) {
	resp, err := c.gcc.GetVariable(c.ctx, &pb.GetVariableRequest{Identifier: &pb.GetVariableRequest_InstanceId{InstanceId: id}})
	if err != nil {
		return nil, grpcError(err)
	}
	v := Variable{client: &c, Variable: *resp.GetVariable()}

	return v.instanceFromID(id), nil
}

// GetVariableFromName returns the variable with that name
func (c Client) GetVariableFromName(name string) (*Variable, error) {
	resp, err := c.gcc.GetVariable(c.ctx, &pb.GetVariableRequest{Identifier: &pb.GetVariableRequest_Name{Name: name}})
	if err != nil {
		return nil, grpcError(err)
	}
	return &Variable{client: &c, Variable: *resp.GetVariable()}, nil
}

// streamListVariables returns a stream of variables that fit the search parameters
func (c Client) streamListVariables(namelike string, limit, page int) (pb.Geocube_ListVariablesClient, error) {
	res, err := c.gcc.ListVariables(c.ctx, &pb.ListVariablesRequest{Name: namelike, Limit: int32(limit), Page: int32(page)})
	return res, grpcError(err)
}

// ListVariables returns a list of variables that fit the search parameters
func (c Client) ListVariables(namelike string, limit, page int) ([]Variable, error) {
	streamVariables, err := c.streamListVariables(namelike, limit, page)
	if err != nil {
		return nil, err
	}
	variables := []Variable{}

	for {
		variable, err := streamVariables.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		variables = append(variables, Variable{client: &c, Variable: *variable.GetVariable()})
	}

	return variables, nil
}

func pbString(value *string) *wrappers.StringValue {
	if value != nil {
		return &wrappers.StringValue{Value: *value}
	}
	return nil
}

// UpdateVariable updates the non-nil fields of the variable
func (c Client) UpdateVariable(id string, name, unit, description, palette, resamplingAlg *string) error {
	if name == nil && unit == nil && description == nil && palette == nil && resamplingAlg == nil {
		return nil
	}
	resampling := pb.Resampling_UNDEFINED
	if resamplingAlg != nil {
		resampling = toResampling(*resamplingAlg)
	}
	_, err := c.gcc.UpdateVariable(c.ctx, &pb.UpdateVariableRequest{
		Id:            id,
		Name:          pbString(name),
		Unit:          pbString(unit),
		Description:   pbString(description),
		Palette:       pbString(palette),
		ResamplingAlg: resampling,
	})
	return grpcError(err)
}

// UpdateInstance updates the non-nil/non-empty fields of the instance
func (c Client) UpdateInstance(instanceID string, name *string, addMetadata map[string]string, delMetadataKeys []string) error {
	if name == nil && len(addMetadata) == 0 && len(delMetadataKeys) == 0 {
		return nil
	}
	_, err := c.gcc.UpdateInstance(c.ctx, &pb.UpdateInstanceRequest{
		Id:              instanceID,
		Name:            pbString(name),
		AddMetadata:     addMetadata,
		DelMetadataKeys: delMetadataKeys,
	})
	return grpcError(err)
}

// DeleteVariable deletes the Variable and all its instances if and only if all instances are pending
func (c Client) DeleteVariable(variableID string) error {
	_, err := c.gcc.DeleteVariable(c.ctx, &pb.DeleteVariableRequest{Id: variableID})
	return grpcError(err)
}

// DeleteInstance deletes the Instance if and only if it's a pending instance (with no datasets)
// DeleteInstance does not delete the Variable
func (c Client) DeleteInstance(instanceID string) error {
	_, err := c.gcc.DeleteInstance(c.ctx, &pb.DeleteInstanceRequest{Id: instanceID})
	return grpcError(err)
}

// CreatePalette defines (or redefines) a palette as a ramp of colors
func (c Client) CreatePalette(name string, colors []ColorPoint, replace bool) error {
	pbcolors := make([]*pb.ColorPoint, len(colors))
	for i := range colors {
		pbcolors[i] = (*pb.ColorPoint)(&colors[i])
	}

	_, err := c.gcc.CreatePalette(c.ctx, &pb.CreatePaletteRequest{
		Palette: &pb.Palette{
			Name:   name,
			Colors: pbcolors,
		},
		Replace: replace,
	})
	return grpcError(err)
}
