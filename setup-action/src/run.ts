// Copyright (c) Jan-Otto Kröpke
// Licensed under the MIT license.

import * as os from 'os';
import * as path from 'path';
import * as fs from 'fs';

import * as toolCache from '@actions/tool-cache';
import * as core from '@actions/core';
import * as http from "@actions/http-client";

const toolName = 'semantic-releaser';
const githubRepo = 'jkroepke/semantic-releaser';
const stableVersion = '0.0.1';

function getExecutableExtension(): string {
    if (os.type().match(/^Win/)) {
        return '.exe';
    }
    return '';
}

function getDownloadURL(version: string): string {
    if (version.toLocaleLowerCase().startsWith('v')) {
        version = version.substring(1);
    }

    let arch = os.arch()
    if (arch === 'x64') {
        arch = 'amd64'
    }
    let osType = os.type().toLowerCase()
    if (osType === 'windows_nt') {
        osType = 'windows'
    }

    return `https://github.com/${githubRepo}/releases/download/v${version}/${toolName}_${version}_${osType}_${arch}.tar.gz`;
}

async function getStableVersion(): Promise<string> {
    try {
        const httpClient = new http.HttpClient()
        const res = await httpClient.getJson<any>(`https://github.com/${githubRepo}/releases/latest`)
        return res.result.tag_name
    } catch (e) {
        core.warning(`Cannot get the latest ${toolName} info from https://github.com/${githubRepo}/releases/latest. Error ${e}. Using default version ${stableVersion}.`);
    }

    return stableVersion;
}

const walkSync = function (dir: string, fileList: string[], fileToFind: string) {
    const files = fs.readdirSync(dir);
    fileList = fileList || [];
    files.forEach(function (file) {
        if (fs.statSync(path.join(dir, file)).isDirectory()) {
            fileList = walkSync(path.join(dir, file), fileList, fileToFind);
        } else {
            core.debug(file);
            if (file === fileToFind) {
                fileList.push(path.join(dir, file));
            }
        }
    });
    return fileList;
};

async function downloadBinary(version: string): Promise<string> {
    if (!version) { version = await getStableVersion(); }
    let cachedToolPath = toolCache.find(toolName, version);
    if (!cachedToolPath) {
        const downloadUrl = getDownloadURL(version);
        let downloadPath: string;
        try {
            downloadPath = await toolCache.downloadTool(downloadUrl);
        } catch (exception) {
            throw new Error(`Failed to download ${toolName} from location ${downloadUrl}`);
        }

        const extractedFolder = await toolCache.extractTar(downloadPath);

        fs.chmodSync(extractedFolder, '777');

        cachedToolPath = await toolCache.cacheFile(extractedFolder + '/' + toolName + getExecutableExtension(), toolName + getExecutableExtension(), toolName, version);
    }

    const binaryPath = findBinary(cachedToolPath);
    if (!binaryPath) {
        throw new Error(`${toolName} executable not found in path ${cachedToolPath}`);
    }

    fs.chmodSync(binaryPath, '777');
    return binaryPath;
}

function findBinary(rootFolder: string): string {
    fs.chmodSync(rootFolder, '777');
    const fileList: string[] = [];
    walkSync(rootFolder, fileList, toolName + getExecutableExtension());
    if (!fileList) {
        throw new Error(`${toolName} executable not found in path ${rootFolder}`);
    }
    else {
        return fileList[0];
    }
}

async function run() {
    let version = core.getInput('version', { 'required': true });
    if (version.toLocaleLowerCase() === 'latest') {
        version = await getStableVersion();
    } else if (!version.toLocaleLowerCase().startsWith('v')) {
        version = 'v' + version;
    }

    const cachedPath = await downloadBinary(version);
    core.addPath(path.dirname(cachedPath))
    core.setOutput(`${toolName}-path`, cachedPath);
}

run().catch(core.setFailed);
