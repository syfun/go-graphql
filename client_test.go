package graphql

import (
	"reflect"
	"testing"
)

type person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestResponse_Guess(t *testing.T) {
	var p person
	var ps []*person
	type fields struct {
		Data   JSON
		Errors []*GraphQLError
		req    *Request
	}
	type args struct {
		name string
		v    interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		v       interface{}
	}{
		// TODO: Add test cases.
		{
			name: "guess struct",
			fields: fields{
				Data: JSON{
					"person": JSON{
						"name": "Jack",
						"age":  26,
					},
				},
			},
			args: args{
				name: "person",
				v:    &p,
			},
			wantErr: false,
			v:       &person{Name: "Jack", Age: 26},
		},
		{
			name: "guess struct slice",
			fields: fields{
				Data: JSON{
					"person": []JSON{
						JSON{
							"name": "Jack",
							"age":  26,
						},
						JSON{
							"name": "Rose",
							"age":  25,
						},
					},
				},
			},
			args: args{
				name: "person",
				v:    &ps,
			},
			wantErr: false,
			v: &[]*person{
				&person{Name: "Jack", Age: 26},
				&person{Name: "Rose", Age: 25},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Response{
				Data:   tt.fields.Data,
				Errors: tt.fields.Errors,
				req:    tt.fields.req,
			}
			if err := r.Guess(tt.args.name, tt.args.v); (err != nil) != tt.wantErr {
				t.Errorf("Response.Guess() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.v, tt.args.v) {
				t.Errorf("Response.Guess() want guess = %v, got %v", tt.v, tt.args.v)
			}
		})
	}
}
