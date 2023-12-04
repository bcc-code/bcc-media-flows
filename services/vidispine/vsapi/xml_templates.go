package vsapi

import "text/template"

var (
	xmlMasterPlaceholderTmpl      = template.Must(template.New("master").Parse(xmlMasterPlaceholder))
	xmlRawMaterialPlaceholderTmpl = template.Must(template.New("raw").Parse(xmlRawMaterialPlaceholder))
	xmlSetMetadataPlaceholderTmpl = template.Must(template.New("metadata").Parse(xmlSetMetadataPlaceholder))
)

const (
	xmlMasterPlaceholder = `<?xml version="1.0"?>
<MetadataDocument
	xmlns="http://xml.vidispine.com/schema/vidispine">
	<group>Master</group>
	<timespan start="-INF" end="+INF">
		<field>
			<name>title</name>
			<value>{{ .Title }}</value>
		</field>
		<!-- Type -->
		<field>
			<name>portal_mf370051</name>
			<value>master</value>
		</field>
		<!-- gruppefelt -->
		<field>
			<name>portal_mf659028</name>
			<value>master</value>
		</field>
		<!-- Status field NEW FIELD-->
		<!-- QC status -->
		<field>
			<name>portal_mf501974</name>
			<value>no</value>
		</field>
		<group>
			<name>Info</name>
			<!-- Supplier -->
			<field>
				<name>portal_mf144377</name>
				<value>btv</value>
			</field>
			<!-- Contact email -->
			<field>
				<name>portal_mf381829</name>
				<value>{{ .Email }}</value>
			</field>
		</group>
	</timespan>
</MetadataDocument>`

	xmlRawMaterialPlaceholder = `<?xml version="1.0"?>
<MetadataDocument
	xmlns="http://xml.vidispine.com/schema/vidispine">
	<group>RawMaterial</group>
	<timespan start="-INF" end="+INF">
		<field>
			<name>title</name>
			<value>{{ .Title }}</value>
		</field>
	</timespan>
</MetadataDocument>`

	xmlSetMetadataPlaceholder = `<?xml version="1.0"?>
<MetadataDocument xmlns="http://xml.vidispine.com/schema/vidispine">
	<timespan start="-INF" end="+INF">
		<field>
			<name>{{.Key}}</name>
			{{if .Add}}
				<value mode="add">{{.Value}}</value>
			{{else}}
				<value>{{.Value}}</value>
			{{end}}
		</field>
	</timespan>
</MetadataDocument>`
)
