"use client";

import React, { useState, useEffect } from 'react';
import { useCookies } from 'react-cookie';
import { GoogleOAuthProvider, GoogleLogin } from '@react-oauth/google';

import {
  GoogleSquareFilled,
  GoogleOutlined,
  AlipayOutlined,
  LockOutlined,
  MobileOutlined,
  TaobaoOutlined,
  UserOutlined,
  WeiboOutlined,
} from '@ant-design/icons';

import {
  LoginFormPage,
  ProConfigProvider,
  ProFormCaptcha,
  ProFormCheckbox,
  ProFormText,
} from '@ant-design/pro-components';
import { Button, Divider, Space, Tabs, message, theme } from 'antd';
import type { CSSProperties } from 'react';

type LoginType = 'azure' | 'gcp';

const iconStyles: CSSProperties = {
  color: 'rgba(0, 0, 0, 0.2)',
  fontSize: '25px',
  verticalAlign: 'middle',
  cursor: 'pointer',
};

const Page = () => {
  const [loginType, setLoginType] = useState<LoginType>('azure');
  const { token } = theme.useToken();
  const [cookies, setCookie] = useCookies(['AUTH_ACCESS_TOKEN'])

  const onGoogleLogin = () => {
    console.log('Starting to login google');
  };

  const handleSuccess = (credentialResponse) => {
    // Handle the successful login here
    console.log('Google login successful', credentialResponse.credential);
    console.log('Google login successful', credentialResponse.clientId);
    console.log('Google login successful', credentialResponse.select_by);

    var base64Url =  credentialResponse.credential.split('.')[1];
    var base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
    var jsonPayload = JSON.parse(decodeURIComponent(window.atob(base64).split('').map(function(c) {
      return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
    }).join('')));
    console.log("payload:", jsonPayload);
    console.log("iss:", jsonPayload["iss"]);
    console.log("azp:", jsonPayload.azp);
    console.log("aud:", jsonPayload.aud);
    console.log("sub:", jsonPayload.sub);
    console.log("email:", jsonPayload.email);
    console.log("email verified:", jsonPayload.email_verified);
    console.log("nbf:", jsonPayload.nbf);
    console.log("name:", jsonPayload.name);
    console.log("picture:", jsonPayload.picture);
    console.log("given_name:", jsonPayload.given_name);
    console.log("family_name:", jsonPayload.family_name);
    console.log("iat:", jsonPayload.iat);
    console.log("exp:", jsonPayload.exp);
    console.log("jti:", jsonPayload.jti);

  };

  const handleError = () => {
    // Handle login errors here
    console.log('Google login failed');
  };


  const onLogin = (values) => {
    console.log(`The login type is : ${loginType}`);
    if (loginType === "azure") {
      azureLogin(values);
    } else if(loginType === "gcp") {
      gcpLogin(values);
    }
  }

  const gcpLogin = (values) => {
    console.log("The login value are ");
    console.log(values);
    var oauth2Endpoint = 'https://accounts.google.com/o/oauth2/v2/auth';

    // Create <form> element to submit parameters to OAuth 2.0 endpoint.
    var form = document.createElement('form');
    form.setAttribute('method', 'GET'); // Send as a GET request.
    form.setAttribute('action', oauth2Endpoint);

    let uuid = "gcp cloud:" + Date.now().toString(36) + Math.random().toString(36).substring(2);
    // Parameters to pass to OAuth 2.0 endpoint.
    var params = {'client_id': '219264688762-h33r9i2oepc4ql2pc1123osvk5e8m7h3.apps.googleusercontent.com',
                  'redirect_uri': 'https://www.51yomo.net/tidbonaks/dashboard',
                  'response_type': 'token',
                  //'scope': 'https://www.googleapis.com/auth/drive.metadata.readonly',
                  'scope': 'https://www.googleapis.com/auth/cloud-platform',
                  'include_granted_scopes': 'true',
                  'state': uuid};
  
    // Add form parameters as hidden input values.
    for (var p in params) {
      var input = document.createElement('input');
      input.setAttribute('type', 'hidden');
      input.setAttribute('name', p);
      input.setAttribute('value', params[p]);
      form.appendChild(input);
    }
  
    // Add form to page and submit it to open the OAuth 2.0 endpoint.
    document.body.appendChild(form);
    form.submit();

  }

  const azureLogin = (values) => {
    console.log("--------------------------"); 
    console.log(values);
    const tenantId = values.tenantId;
    const clientId = values.clientId;

    setCookie('TENANT_ID', tenantId, { path: '/'});
    setCookie('CLIENT_ID', clientId, { path: '/'});
    
    let uuid = "az cloud:" + Date.now().toString(36) + Math.random().toString(36).substring(2);

    location.href=`https://login.microsoftonline.com/${values.tenantId}/oauth2/v2.0/authorize?client_id=${values.clientId}&response_type=token&response_mode=fragment&state=${uuid}&scope=https://management.azure.com/user_impersonation&redirect_uri=https://www.51yomo.net/tidbonaks/dashboard&nonce=678910`
  }

  const items: TabsProps['items'] = [
    {
      key: 'azure',
      label: 'Azure',
    },
    {
      key: 'gcp',
      label: 'GCP',
    }
  ];

  return (
    <div style={{ backgroundColor: 'white', height: '100vh' }} >
      <LoginFormPage
        backgroundImageUrl="https://images.pexels.com/photos/1166644/pexels-photo-1166644.jpeg?auto=compress&cs=tinysrgb&w=1260&h=750&dpr=2"
        logo="https://cdn.iconscout.com/icon/free/png-512/free-geek-icon-download-in-svg-png-gif-file-formats--brainy-studious-specs-cool-kiddo-pack-sports-games-icons-160919.png"
        backgroundVideoUrl="https://gw.alipayobjects.com/v/huamei_gcee1x/afts/video/jXRBRK_VAwoAAAAAAAAAAAAAK4eUAQBr"
        title="Hello World"
        containerStyle={{ backgroundColor: 'rgba(0, 0, 0, 0.6)', backdropFilter: 'blur(4px)' }}
        subTitle="AZURE"
        onFinish={ onLogin }
        actions={
          <GoogleOAuthProvider clientId={ process.env.GAPI_CLIENT_ID} >
            <GoogleLogin onSuccess={handleSuccess} onError={handleError} />
          </GoogleOAuthProvider>
        }
      >
        <>
          <Tabs centered activeKey={loginType} onChange={(activeKey) => setLoginType(activeKey as LoginType)} items={items} />

          <ProFormText name="tenantId" value={ cookies.TENANT_ID }
            fieldProps={{ size: 'large', prefix: ( <UserOutlined style={{ color: token.colorText }} className={'prefixIcon'} />), }}
            placeholder={'Tenant ID: tenant id'}
            rules={[ { required: true, message: 'Please input tenant id' } ]}
          />
          <ProFormText name="clientId" value={ cookies.CLIENT_ID }
            fieldProps={{ size: 'large', prefix: ( <UserOutlined style={{ color: token.colorText }} className={'prefixIcon'} />), }}
            placeholder={'Client ID: client id'}
            rules={[ { required: true, message: 'Please input client id' } ]}
          />
        </>
      </LoginFormPage>
    </div>
  );
};

export default () => {
  return (
    <ProConfigProvider dark>
      <Page />
    </ProConfigProvider>
  );
};
