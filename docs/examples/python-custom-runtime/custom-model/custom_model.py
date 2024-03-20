import os
from os.path import exists
from typing import Dict, List
from mlserver import MLModel
from mlserver.utils import get_model_uri
from mlserver.types import InferenceRequest, InferenceResponse, ResponseOutput, Parameters
from mlserver.codecs import DecodedParameterName
from joblib import load

import logging
import numpy as np

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

_to_exclude = {
    "parameters": {DecodedParameterName, "headers"},
    'inputs': {"__all__": {"parameters": {DecodedParameterName, "headers"}}}
}

WELLKNOWN_MODEL_FILENAMES = ["mnist-svm.joblib"]


class CustomMLModel(MLModel):  # pylint:disable=c-extension-no-member
    async def load(self) -> bool:
        model_uri = await get_model_uri(self._settings, wellknown_filenames=WELLKNOWN_MODEL_FILENAMES)
        logger.info("Model load URI: {model_uri}")
        if exists(model_uri):
            logger.info(f"Loading MNIST model from {model_uri}")
            self._model = load(model_uri)
            logger.info("Model loaded successfully")
        else:
            logger.info(f"Model not exist in {model_uri}")
            # raise FileNotFoundError(model_uri)
            self.ready = False
            return self.ready
    
        self.ready = True
        return self.ready

    async def predict(self, payload: InferenceRequest) -> InferenceResponse:
        input_data = [input_data.data for input_data in payload.inputs]
        input_name = [input_data.name for input_data in payload.inputs]
        input_data_array = np.array(input_data)
        result = self._model.predict(input_data_array) 
        predictions = np.array(result)

        logger.info(f"Predict result is: {result}")
        return InferenceResponse(
            id=payload.id,
            model_name = self.name,
            model_version = self.version,
            outputs = [
                ResponseOutput(
                    name = str(input_name[0]),
                    shape = predictions.shape,
                    datatype = "INT64",
                    data=predictions.tolist(),
                )
            ],
        )
